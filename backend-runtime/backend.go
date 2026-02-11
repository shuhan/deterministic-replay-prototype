package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RecordType string

const (
	RequestRecordType            RecordType = "request"
	ResponseRecordType           RecordType = "response"
	DependencyRequestRecordType  RecordType = "dependency-request"
	DependencyResponseRecordType RecordType = "dependency-response"
)

type Record struct {
	RequestContext     string              `json:"rc"`
	CauseContext       string              `json:"cc"`
	ExecutionContext   string              `json:"ec"`
	DependencyContext  string              `json:"dc"`
	RecordType         RecordType          `json:"rt"`
	Method             string              `json:"rm"`
	Time               time.Time           `json:"tm"`
	Duration           int64               `json:"dr"`
	DepencencySequence int                 `json:"dq"`
	ScopedSequence     int                 `json:"sq"`
	ServiceName        string              `json:"sn"`
	Host               string              `json:"rh"`
	Uri                string              `json:"ru"`
	Header             map[string][]string `json:"he"`
	Body               []byte              `json:"bd"`
	StatusCode         int                 `json:"st"`
}

type Request struct {
	In           Record       `json:"in"`
	Dependencies []Dependency `json:"dep"`
	Out          Record       `json:"out"`
}

type Dependency struct {
	In        Record  `json:"in"`
	Out       Record  `json:"out"`
	Reference Request `json:"ref"`
}

var (
	data  map[string][]Record
	rwMux sync.RWMutex
)

func main() {
	fmt.Println("Starting backend runtime")

	data = make(map[string][]Record)

	http.HandleFunc("/runtime/record", recordHandler)
	http.HandleFunc("/runtime/replay", replayHandler)
	http.HandleFunc("/runtime/proxy", proxyHandler)

	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

func recordHandler(w http.ResponseWriter, r *http.Request) {
	var (
		badRequest bool
		oErr       error
	)

	defer func() {
		if badRequest {
			w.WriteHeader(http.StatusBadRequest)
		} else if oErr != nil {
			fmt.Printf("ERROR: %s\n", oErr.Error())
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()

	if r.Method != "POST" {
		badRequest = true
		return
	}

	if len(r.Header["Content-Type"]) == 0 || r.Header["Content-Type"][0] != "application/json" {
		badRequest = true
		return
	}

	if r.ContentLength == 0 {
		badRequest = true
		return
	}

	body, oErr := io.ReadAll(r.Body)
	if oErr != nil {
		return
	}

	rwMux.Lock()
	defer rwMux.Unlock()
	records := make([]Record, 0)

	if oErr = json.Unmarshal(body, &records); oErr != nil {
		return
	}

	for _, rc := range records {
		data[rc.RequestContext] = append(data[rc.RequestContext], rc)
	}
	w.WriteHeader(http.StatusAccepted)
}

func replayHandler(w http.ResponseWriter, r *http.Request) {
	var (
		badRequest bool
		oErr       error
	)

	defer func() {
		if badRequest {
			w.WriteHeader(http.StatusBadRequest)
		} else if oErr != nil {
			fmt.Printf("ERROR: %s\n", oErr.Error())
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()

	if r.Method != "GET" {
		badRequest = true
		return
	}

	queries := r.URL.Query()
	rc := queries.Get("rc")

	if rc == "" {
		badRequest = true
		return
	}

	rwMux.RLock()
	defer rwMux.RUnlock()
	if records, ok := data[rc]; ok {

		ec := ""
		// Find the initial request
		for _, r := range records {
			// Records of inital request has cause context as request context
			if r.RequestContext == rc && r.CauseContext == rc {
				ec = r.ExecutionContext
				break
			}
		}

		req := buildRequestTree(records, ec)
		if body, oErr := json.Marshal(req); oErr == nil {
			w.Header().Add("Content-Type", "application/json")
			_, oErr = w.Write(body)
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func buildRequestTree(records []Record, ec string) Request {
	req := Request{
		Dependencies: make([]Dependency, len(records)),
	}

	notUsed := make([]Record, 0, len(records))

	maxGsq := -1

	for i := range records {
		if records[i].ExecutionContext == ec {
			switch records[i].RecordType {
			case RequestRecordType:
				req.In = records[i]
				break
			case ResponseRecordType:
				req.Out = records[i]
				break
			case DependencyRequestRecordType:
				req.Dependencies[records[i].DepencencySequence].In = records[i]
				if records[i].DepencencySequence > maxGsq {
					maxGsq = records[i].DepencencySequence
				}
				break
			case DependencyResponseRecordType:
				req.Dependencies[records[i].DepencencySequence].Out = records[i]
				if records[i].DepencencySequence > maxGsq {
					maxGsq = records[i].DepencencySequence
				}
				break
			default:
				fmt.Printf("Unknown record %v\n", records[i])
			}
		} else {
			notUsed = append(notUsed, records[i])
		}
	}

	req.Dependencies = req.Dependencies[0 : maxGsq+1]

	for i := range req.Dependencies {
		if req.Dependencies[i].In.DependencyContext != "" {
			req.Dependencies[i].Reference = buildRequestTree(notUsed, req.Dependencies[i].In.DependencyContext)
		}
	}

	return req
}

const (
	RequestContextHeader           = "X-Request-Context"
	CauseContextHeader             = "X-Cause-Context"
	ExecutionContextHeader         = "X-Execute-Context"
	ServiceDebugHeader             = "X-Service-Debug"
	DebugConfigHeader              = "X-Debug-Config"
	DepencencySequenceHeader       = "X-Dependency-Sequence"
	ScopedDependencySequenceHeader = "X-Scoped-Dependency-Sequence"

	DebugEnabled = "ENABLED"
)

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	var (
		badRequest bool
		oErr       error
	)

	defer func() {
		if badRequest {
			w.WriteHeader(http.StatusBadRequest)
		} else if oErr != nil {
			fmt.Printf("ERROR: %s\n", oErr.Error())
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()

	queries := r.URL.Query()
	originalUrl := queries.Get("ref")

	rc := r.Header.Get(RequestContextHeader)
	cc := r.Header.Get(CauseContextHeader)
	ss := r.Header.Get(ScopedDependencySequenceHeader)
	dc := r.Header.Get(DebugConfigHeader)

	seq, err := strconv.Atoi(ss)
	if err != nil {
		badRequest = true
		oErr = err
		return
	}

	mapping := parseDebugConfig(dc)

	rwMux.RLock()
	defer rwMux.RUnlock()
	if records, ok := data[rc]; ok {
		var depRes, depInReq Record
		for _, rec := range records {
			if rec.RecordType == DependencyResponseRecordType && rec.ExecutionContext == cc && rec.Uri == originalUrl && rec.ScopedSequence == seq {
				depRes = rec
				break
			}
		}

		for _, rec := range records {
			if rec.RecordType == RequestRecordType && rec.ExecutionContext == depRes.DependencyContext {
				depInReq = rec
				break
			}
		}

		if host, ok := mapping[strings.ToLower(depInReq.ServiceName)]; ok {
			// forward request
			reqUrl, err := url.Parse(originalUrl)
			if err != nil {
				oErr = err
				return
			}
			reqUrl.Host = host
			if strings.HasPrefix(host, "localhost") {
				reqUrl.Scheme = "http"
			} else {
				reqUrl.Scheme = "https"
			}

			req, err := http.NewRequest(r.Method, reqUrl.String(), r.Body)
			if err != nil {
				oErr = err
				return
			}

			for name, val := range depInReq.Header {
				if len(val) > 0 {
					req.Header.Add(name, val[0])
				}
			}

			req.Header.Set(RequestContextHeader, depInReq.RequestContext)
			req.Header.Set(CauseContextHeader, depInReq.CauseContext)
			req.Header.Set(ExecutionContextHeader, depInReq.ExecutionContext)
			req.Header.Set(ServiceDebugHeader, DebugEnabled)
			req.Header.Set(DebugConfigHeader, dc)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				oErr = err
				return
			}

			for name, val := range resp.Header {
				if len(val) > 0 {
					w.Header().Add(name, val[0])
				}
			}
			w.WriteHeader(resp.StatusCode)
			if resp.ContentLength > 0 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					oErr = err
					return
				}
				w.Write(body)
			}

		} else {
			// Forward snapshot
			for name, val := range depRes.Header {
				if len(val) > 0 {
					w.Header().Add(name, val[0])
				}
			}
			w.WriteHeader(depRes.StatusCode)
			if len(depRes.Body) > 0 {
				w.Write(depRes.Body)
			}
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func parseDebugConfig(config string) map[string]string {
	shs := strings.Split(config, "|")

	retval := make(map[string]string, len(shs))

	for _, s := range shs {
		sh := strings.Split(s, "=")
		if len(sh) == 2 {
			retval[sh[0]] = sh[1]
		}
	}

	return retval
}

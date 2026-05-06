package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
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
	ObservedRecordType           RecordType = "observed"
)

type Record struct {
	RequestContext      string              `json:"rc"`
	CauseContext        string              `json:"cc"`
	ExecutionContext    string              `json:"ec"`
	DependencyContext   string              `json:"dc"`
	RecordType          RecordType          `json:"rt"`
	Method              string              `json:"rm"`
	Time                time.Time           `json:"tm"`
	Duration            int64               `json:"dr"`
	DepencencySequence  int                 `json:"dq"`
	ScopedSequence      int                 `json:"sq"`
	ObservationSequence int                 `json:"oq"`
	ServiceName         string              `json:"sn"`
	ObservationName     string              `json:"on"`
	Host                string              `json:"rh"`
	Uri                 string              `json:"ru"`
	Header              map[string][]string `json:"he"`
	Body                []byte              `json:"bd"`
	ObservationError    []byte              `json:"oe"`
	StatusCode          int                 `json:"st"`
}

type Request struct {
	In           Record       `json:"in"`
	Dependencies []Dependency `json:"dep"`
	Observations []Record     `json:"ob"`
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

	http.HandleFunc("/record", recordHandler)
	http.HandleFunc("/list", listHandler)
	http.HandleFunc("/get", getHandler)

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

func listHandler(w http.ResponseWriter, r *http.Request) {
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
	sr := queries.Get("sr") // Service names ; seperated
	ex := queries.Get("ex") // Excluded request contexts ; seperated
	st := queries.Get("st") // HTTP Status Codes ; seperated
	n := queries.Get("n")

	if sr == "" {
		badRequest = true
		return
	}

	serviceNames := strings.Split(sr, "|")

	for i := range serviceNames {
		serviceNames[i] = strings.ToLower(serviceNames[i])
	}

	MaxCount := 10

	if n != "" {
		MaxCount, oErr = strconv.Atoi(n)
		if oErr != nil {
			badRequest = true
			return
		}
	}

	statusFilterEnabled := st != ""
	statuses := []int{}

	if statusFilterEnabled {
		sts := strings.SplitSeq(st, "|")

		for s := range sts {
			status, err := strconv.Atoi(s)
			if err != nil {
				oErr = err
				return
			}
			statuses = append(statuses, status)
		}
	}

	excludes := []string{}

	if ex != "" {
		excludes = strings.Split(ex, "|")
	}

	rwMux.RLock()
	defer rwMux.RUnlock()

	resp := []string{}

Itr:
	for key, recs := range data {

		hasService := false

		for ri := range recs {
			if inArray(serviceNames, strings.ToLower(recs[ri].ServiceName)) {
				hasService = true
			}

			if statusFilterEnabled {
				if recs[ri].RecordType == DependencyResponseRecordType || recs[ri].RecordType == ResponseRecordType {
					if !inArray(statuses, recs[ri].StatusCode) {
						continue Itr
					}
				}
			}
		}

		if !hasService {
			continue
		}

		if !inArray(excludes, key) {
			resp = append(resp, key)
			if len(resp) == MaxCount {
				break
			}
		}
	}

	if body, oErr := json.Marshal(resp); oErr == nil {
		w.Header().Add("Content-Type", "application/json")
		_, oErr = w.Write(body)
	}
}

func inArray[T any](haystack []T, niddle T) bool {
	if len(haystack) == 0 {
		return false
	}

	for _, h := range haystack {
		if reflect.DeepEqual(h, niddle) {
			return true
		}
	}
	return false
}

func getHandler(w http.ResponseWriter, r *http.Request) {
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
		if body, oErr := json.Marshal(records); oErr == nil {
			w.Header().Add("Content-Type", "application/json")
			_, oErr = w.Write(body)
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

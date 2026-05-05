package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

type CloseFunc func()

var (
	data      map[string][]Record
	rwMux     sync.RWMutex
	DebugHost string
)

func getDebugRecords(rc string) []Record {
	rwMux.RLock()
	defer rwMux.RUnlock()

	if records, ok := data[rc]; ok {
		return records
	}

	return nil
}

func addDebugData(rc string, records []Record) {
	rwMux.Lock()
	defer rwMux.Unlock()
	if data == nil {
		data = make(map[string][]Record)
	}
	data[rc] = records
}

func startDebugHost() (CloseFunc, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return func() {}, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	DebugHost = fmt.Sprintf("http://localhost:%d", port)

	go func() {
		http.HandleFunc("/proxy", proxyHandler)
		http.HandleFunc("/observations", observationHandler)
		http.Serve(listener, nil)
	}()

	fmt.Printf("Debug host listening to: %s\n", DebugHost)

	return func() {
		listener.Close()
	}, nil
}

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
			req.Header.Set(DebugHostHeader, DebugHost)

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

type ObservationData struct {
	Body             []byte `json:"bd"`
	ObservationError []byte `json:"oe"`
}

type Observations struct {
	Data map[string]map[int]ObservationData `json:"data"`
}

func observationHandler(w http.ResponseWriter, r *http.Request) {
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

	rc := r.Header.Get(RequestContextHeader)
	dc := r.Header.Get(DebugConfigHeader)

	mapping := parseDebugConfig(dc)

	rwMux.RLock()
	defer rwMux.RUnlock()
	if records, ok := data[rc]; ok {

		obs := Observations{
			Data: make(map[string]map[int]ObservationData, len(records)),
		}

		for _, rec := range records {
			if rec.RecordType == ObservedRecordType {
				mappingKey := strings.ToLower(rec.ServiceName + ":" + rec.ObservationName)
				if mapped, ok := mapping[mappingKey]; !(ok && mapped == "pass") {
					if _, ok := obs.Data[rec.ObservationName]; !ok {
						obs.Data[rec.ObservationName] = make(map[int]ObservationData)
					}
					obs.Data[rec.ObservationName][rec.ScopedSequence] = ObservationData{Body: rec.Body, ObservationError: rec.ObservationError}
				}
			}
		}

		data, err := json.Marshal(obs)
		if err != nil {
			oErr = err
			return
		}

		w.Write(data)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

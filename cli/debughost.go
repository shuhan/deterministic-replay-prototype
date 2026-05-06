package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

type CloseFunc func()

var (
	DebugHost                  string
	dependencyRegressionPassed bool
	allowRequestDiversion      bool
)

func ResetDependencyRegression() {
	dependencyRegressionPassed = true
}

func hasDependencyRegressionPassed() bool {
	return dependencyRegressionPassed
}

func StartDebugHost(allowDiversion bool) (CloseFunc, error) {
	allowRequestDiversion = allowDiversion
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return func() {}, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	DebugHost = fmt.Sprintf("http://localhost:%d", port)

	waitFor := make(chan any)

	go func() {
		defer func() {
			fmt.Println("Debug host closed")
			close(waitFor)
		}()
		http.HandleFunc("/proxy", proxyHandler)
		http.HandleFunc("/observations", observationHandler)
		http.Serve(listener, nil)
	}()

	fmt.Printf("Debug host listening to: %s\n", DebugHost)

	return func() {
		listener.Close()
		<-waitFor
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

	mapping := ParseDebugConfig(dc)

	if records, err := GetRecords(rc); err == nil {
		var depRes, depInReq Record
		for _, rec := range records {
			if rec.RecordType == DependencyResponseRecordType && rec.ExecutionContext == cc && rec.Uri == originalUrl && rec.ScopedSequence == seq {
				depRes = rec
				break
			}
		}

		var body []byte

		// here if depRes is not set, we failed to resolve dependency call so we
		if reflect.ValueOf(depRes).IsZero() {
			dependencyRegressionPassed = false
			fmt.Printf("Request diversed at this point, URL: %s was not recorded\n", originalUrl)
			if allowRequestDiversion {
				// shall we pass through with warning
				fmt.Println("Request being diverted to source")
				req, err := http.NewRequest(r.Method, originalUrl, r.Body)
				if err != nil {
					oErr = err
					return
				}

				for name, val := range r.Header {
					if len(val) > 0 {
						req.Header.Add(name, val[0])
					}
				}

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
				fmt.Println("Request diversion not allowed")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Request diversion not allowed"))
			}
			return
		} else {

			for _, rec := range records {
				if rec.RecordType == RequestRecordType && rec.ExecutionContext == depRes.DependencyContext {
					depInReq = rec
					break
				}
			}
			if r.Body != nil {
				body, err = io.ReadAll(r.Body)
				if err != nil {
					oErr = err
					return
				}
			}

			if r.Method != depInReq.Method || !bytes.Equal(body, depInReq.Body) {
				dependencyRegressionPassed = false
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

			var requestBody io.Reader

			if body != nil {
				requestBody = bytes.NewBuffer(body)
			}

			req, err := http.NewRequest(r.Method, reqUrl.String(), requestBody)
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
		fmt.Println(err)
		w.WriteHeader(http.StatusNotFound)
	}
}

type observationData struct {
	Body             []byte `json:"bd"`
	ObservationError []byte `json:"oe"`
}

type observations struct {
	Data map[string]map[int]observationData `json:"data"`
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

	mapping := ParseDebugConfig(dc)

	if records, err := GetRecords(rc); err == nil {
		obs := observations{
			Data: make(map[string]map[int]observationData, len(records)),
		}

		for _, rec := range records {
			if rec.RecordType == ObservedRecordType {
				mappingKey := strings.ToLower(rec.ServiceName + ":" + rec.ObservationName)
				if mapped, ok := mapping[mappingKey]; !(ok && mapped == "pass") {
					if _, ok := obs.Data[rec.ObservationName]; !ok {
						obs.Data[rec.ObservationName] = make(map[int]observationData)
					}
					obs.Data[rec.ObservationName][rec.ScopedSequence] = observationData{Body: rec.Body, ObservationError: rec.ObservationError}
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
		fmt.Println(err)
		w.WriteHeader(http.StatusNotFound)
	}
}

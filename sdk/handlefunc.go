package sdk

import (
	"context"
	"net/http"
	"time"
)

func HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	http.HandleFunc(pattern, WithAudit(handler))
}

func WithAudit(handler func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		serviceContext, err := NewServiceContext(r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid context"))
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), serviceContextKey, serviceContext))
		bodyBytes := readAndRestore(&r.Body)
		rw := NewResponseWritter(w, serviceContext)

		if !serviceContext.Debug {
			go func() {
				Log(Record{
					RequestContext:     serviceContext.RequestContext,
					CauseContext:       serviceContext.CauseContext,
					ExecutionContext:   serviceContext.ExecutionContext,
					RecordType:         RequestRecordType,
					Method:             r.Method,
					Time:               start,
					Duration:           0,
					DepencencySequence: 0,
					ScopedSequence:     0,
					ServiceName:        serviceName,
					Host:               r.Host,
					Uri:                r.URL.String(),
					Header:             r.Header,
					Body:               bodyBytes,
					StatusCode:         0,
				})
			}()
		}

		defer func() {
			duration := time.Since(start).Milliseconds()
			if !serviceContext.Debug {
				go func() {
					Log(Record{
						RequestContext:     serviceContext.RequestContext,
						CauseContext:       serviceContext.CauseContext,
						ExecutionContext:   serviceContext.ExecutionContext,
						RecordType:         ResponseRecordType,
						Method:             r.Method,
						Time:               start,
						Duration:           duration,
						DepencencySequence: 0,
						ScopedSequence:     0,
						ServiceName:        serviceName,
						Host:               r.Host,
						Uri:                r.URL.String(),
						Header:             rw.headers,
						Body:               rw.buffer,
						StatusCode:         rw.status,
					})
				}()
			}
		}()

		handler(rw, r)
		if !rw.written {
			rw.WriteHeader(rw.status)
		}
	}
}

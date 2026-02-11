package sdk

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Transport struct {
	Base http.RoundTripper
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	sc, ok := req.Context().Value(serviceContextKey).(*ServiceContext)

	if !ok {
		return nil, fmt.Errorf("Request missing tracing context, please use http.NewRequestWithContext() to create the request")
	}

	gsq := sc.GlobalDependencySequence()
	seq := sc.ScopedDependencySequence(req)

	dependencyContext := sc.NewExecutionID()

	// inject headers for downstream services
	req.Header.Set(RequestContextHeader, sc.RequestContext)   // Request context propagates as is
	req.Header.Set(CauseContextHeader, sc.ExecutionContext)   // Current execution is dependencies Cause for execution
	req.Header.Set(ExecutionContextHeader, dependencyContext) // Each dependency call get't it's own unique execution context
	if sc.Debug {
		req.Header.Set(ServiceDebugHeader, DebugEnabled)
		req.Header.Set(DebugConfigHeader, sc.DebugConfig)
		req.Header.Set(DepencencySequenceHeader, strconv.Itoa(gsq))
		req.Header.Set(ScopedDependencySequenceHeader, strconv.Itoa(seq))
		// DEBUG mode: If debug mode is enabled, we replace the URL with debug URL and let debug host decide what to do with it
		debugUrl, err := url.Parse(sc.DebugHost)
		if err != nil {
			return nil, err
		}
		q := debugUrl.Query()
		q.Add("ref", req.URL.String())
		debugUrl.RawQuery = q.Encode()
		req.URL = debugUrl
	}

	// capture outbound request body (bounded) while preserving for transport
	outBody := readAndRestore(&req.Body)

	start := time.Now()

	if !sc.Debug {
		go func() {
			Log(Record{
				RequestContext:     sc.RequestContext,
				CauseContext:       sc.CauseContext,
				ExecutionContext:   sc.ExecutionContext,
				DependencyContext:  dependencyContext,
				RecordType:         DependencyRequestRecordType,
				Method:             req.Method,
				Time:               start,
				Duration:           0,
				DepencencySequence: gsq,
				ScopedSequence:     seq,
				ServiceName:        serviceName,
				Host:               req.Host,
				Uri:                req.URL.String(),
				Header:             req.Header,
				Body:               outBody,
				StatusCode:         0,
			})
		}()
	}

	resp, err := t.Base.RoundTrip(req)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		if !sc.Debug {
			go func() {
				Log(Record{
					RequestContext:     sc.RequestContext,
					CauseContext:       sc.CauseContext,
					ExecutionContext:   sc.ExecutionContext,
					DependencyContext:  dependencyContext,
					RecordType:         DependencyResponseRecordType,
					Method:             req.Method,
					Time:               start,
					Duration:           duration,
					DepencencySequence: gsq,
					ScopedSequence:     seq,
					ServiceName:        serviceName,
					Host:               req.Host,
					Uri:                req.URL.String(),
					Header:             nil,
					Body:               nil,
					StatusCode:         resp.StatusCode,
				})
			}()
		}
		return nil, err
	}

	// capture response body (bounded) while preserving for caller
	respBody := readAndRestore(&resp.Body)

	if !sc.Debug {
		go func() {
			Log(Record{
				RequestContext:     sc.RequestContext,
				CauseContext:       sc.CauseContext,
				ExecutionContext:   sc.ExecutionContext,
				DependencyContext:  dependencyContext,
				RecordType:         DependencyResponseRecordType,
				Method:             req.Method,
				Time:               start,
				Duration:           duration,
				DepencencySequence: gsq,
				ScopedSequence:     seq,
				ServiceName:        serviceName,
				Host:               req.Host,
				Uri:                req.URL.String(),
				Header:             resp.Header,
				Body:               respBody,
				StatusCode:         resp.StatusCode,
			})
		}()
	}

	// restore resp.Body already done by readAndRestore
	return resp, nil
}

func InstrumentClient(c *http.Client) {
	if c.Transport == nil {
		c.Transport = http.DefaultTransport
	}
	c.Transport = &Transport{Base: c.Transport}
}

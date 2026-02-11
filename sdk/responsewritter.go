package sdk

import (
	"net/http"
	"strconv"
)

type ResponseWritter struct {
	headers        http.Header
	status         int
	buffer         []byte
	orig           http.ResponseWriter
	serviceContext *ServiceContext
	written        bool
}

func NewResponseWritter(original http.ResponseWriter, serviceContext *ServiceContext) *ResponseWritter {
	return &ResponseWritter{
		orig:           original,
		serviceContext: serviceContext,
		status:         http.StatusOK,
		written:        false,
	}
}

func (w *ResponseWritter) Header() http.Header {
	return w.orig.Header()
}

func (w *ResponseWritter) addContextHeaders() {
	if w.headers == nil {
		w.headers = w.orig.Header().Clone()
		w.orig.Header().Add(RequestContextHeader, w.serviceContext.RequestContext)
		w.orig.Header().Add(CauseContextHeader, w.serviceContext.CauseContext)
		w.orig.Header().Add(ExecutionContextHeader, w.serviceContext.ExecutionContext)
	}
}

func (w *ResponseWritter) Write(data []byte) (int, error) {
	w.addContextHeaders()
	w.buffer = append(w.buffer, data...)
	w.headers.Set("Content-Length", strconv.Itoa(len(w.buffer)))
	w.written = true
	return w.orig.Write(data)
}

func (w *ResponseWritter) WriteHeader(statusCode int) {
	w.addContextHeaders()
	w.status = statusCode
	w.written = true
	w.orig.WriteHeader(statusCode)
}

package sdk

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type contextKey string

const (
	serviceContextKey = contextKey("service-context")

	RequestContextHeader           = "X-Request-Context"
	CauseContextHeader             = "X-Cause-Context"
	ExecutionContextHeader         = "X-Execute-Context"
	ServiceDebugHeader             = "X-Service-Debug"
	DebugConfigHeader              = "X-Debug-Config"
	DepencencySequenceHeader       = "X-Dependency-Sequence"
	ScopedDependencySequenceHeader = "X-Scoped-Dependency-Sequence"

	DebugEnabled = "ENABLED"
)

type ServiceContext struct {
	RequestContext     string
	CauseContext       string
	ExecutionContext   string
	Debug              bool
	DebugConfig        string // ServiceName:Hostname|ServiceName:Hostname tells debug host how to route requests
	DebugHost          string
	depencencySequence int
	scopedSequenc      map[string]int
}

func (sc *ServiceContext) NewExecutionID() string {
	return uuid.NewString()
}

func (sc *ServiceContext) GlobalDependencySequence() int {
	retval := sc.depencencySequence
	sc.depencencySequence++
	return retval
}

func (sc *ServiceContext) ScopedDependencySequence(request *http.Request) int {
	key := request.URL.String()

	if i := strings.Index(key, "?"); i != -1 {
		key = key[:i]
	} else if i := strings.Index(key, "#"); i != -1 {
		key = key[:i]
	}

	retval := sc.scopedSequenc[key]
	sc.scopedSequenc[key]++
	return retval
}

func NewServiceContext(r *http.Request) (*ServiceContext, error) {
	s := &ServiceContext{
		RequestContext:     r.Header.Get(RequestContextHeader),
		CauseContext:       r.Header.Get(CauseContextHeader),
		ExecutionContext:   r.Header.Get(ExecutionContextHeader),
		DebugConfig:        r.Header.Get(DebugConfigHeader),
		Debug:              r.Header.Get(ServiceDebugHeader) == DebugEnabled,
		depencencySequence: 0,
		scopedSequenc:      map[string]int{},
	}

	if s.Debug {
		s.DebugHost = debugHost
	}

	// Edge request
	if s.RequestContext == "" {
		s.RequestContext = uuid.NewString()
		s.CauseContext = s.RequestContext
		s.ExecutionContext = uuid.NewString()
	}

	if s.CauseContext == "" || s.ExecutionContext == "" {
		return s, fmt.Errorf("Internal request missing context")
	}

	if s.Debug && s.DebugHost == "" {
		return s, fmt.Errorf("invalid debug request")
	}

	return s, nil
}

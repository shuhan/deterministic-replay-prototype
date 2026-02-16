package sdk

import (
	"encoding/json"
	"fmt"
	"io"
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

type ObservationData struct {
	Body             []byte `json:"bd"`
	ObservationError []byte `json:"oe"`
}

type Observations struct {
	Data map[string]map[int]ObservationData `json:"data"`
}

type ServiceContext struct {
	RequestContext      string
	CauseContext        string
	ExecutionContext    string
	Debug               bool
	DebugConfig         string // ServiceName:Hostname|ServiceName:Hostname tells debug host how to route requests
	DebugHost           string
	depencencySequence  int
	scopedSequenc       map[string]int
	observationSequence int
	observationData     map[string]map[int]ObservationData
	waiter              <-chan interface{}
}

func (sc *ServiceContext) NewExecutionID() string {
	return uuid.NewString()
}

func (sc *ServiceContext) GlobalDependencySequence() int {
	retval := sc.depencencySequence
	sc.depencencySequence++
	return retval
}

func (sc *ServiceContext) RequestScopedDependencySequence(request *http.Request) int {
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

func (sc *ServiceContext) ObservationScopedDependencySequence(key string) int {
	retval := sc.scopedSequenc[key]
	sc.scopedSequenc[key]++
	return retval
}

func (sc *ServiceContext) ObservationSequence() int {
	retval := sc.observationSequence
	sc.observationSequence++
	return retval
}

func (sc *ServiceContext) ObservationData(key string, seq int) (ObservationData, bool) {
	if sc.waiter != nil {
		<-sc.waiter
		sc.waiter = nil
	}
	if vals, ok := sc.observationData[key]; ok {
		if val, ok := vals[seq]; ok {
			return val, true
		}
	}
	return ObservationData{}, false
}

var ObserverClient = &http.Client{}

func (sc *ServiceContext) LoadObservations() {
	waiter := make(chan interface{})

	go func() {
		req, err := http.NewRequest(http.MethodGet, observerHost, nil)
		if err != nil {
			fmt.Printf("Error creating observation request: %s\n", err.Error())
			close(waiter)
			return
		}
		req.Header.Set(RequestContextHeader, sc.RequestContext)
		req.Header.Set(DebugConfigHeader, sc.DebugConfig)
		resp, err := ObserverClient.Do(req)
		if err != nil {
			fmt.Printf("Error requesting observation data: %s\n", err.Error())
			close(waiter)
			return
		}

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Observation response error, status code: %d\n", resp.StatusCode)
			close(waiter)
			return
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Error reading observation data: %s\n", err.Error())
			close(waiter)
			return
		}

		obs := Observations{}
		err = json.Unmarshal(data, &obs)
		if err != nil {
			fmt.Printf("Error unmarshalling observation data: %s\n", err.Error())
			close(waiter)
			return
		}
		sc.observationData = obs.Data
		close(waiter)
	}()

	sc.waiter = waiter
}

func NewServiceContext(r *http.Request) (*ServiceContext, error) {
	s := &ServiceContext{
		RequestContext:      r.Header.Get(RequestContextHeader),
		CauseContext:        r.Header.Get(CauseContextHeader),
		ExecutionContext:    r.Header.Get(ExecutionContextHeader),
		DebugConfig:         r.Header.Get(DebugConfigHeader),
		Debug:               r.Header.Get(ServiceDebugHeader) == DebugEnabled,
		depencencySequence:  0,
		scopedSequenc:       map[string]int{},
		observationSequence: 0,
	}

	if s.Debug {
		s.DebugHost = debugHost
		s.LoadObservations()
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

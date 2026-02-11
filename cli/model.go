package main

import "time"

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

type Action string

const (
	ShowAction   = Action("show")
	ReplayAction = Action("replay")
)

type Input struct {
	Action         Action
	RequestContext string
	Mapping        map[string]string
}

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

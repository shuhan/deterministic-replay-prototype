package sdk

import "time"

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
	StatusCode          int                 `json:"st"`
}

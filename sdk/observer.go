package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Observer is an state observer that looks at and returns a state
// In debug mode it will injected and observation from recorded execution
type Observer[T any] interface {
	Observe(ctx context.Context, value T) T
}

type StateObserver[T any] struct {
	name string
}

func (o *StateObserver[T]) Observe(ctx context.Context, value T) T {
	sc, ok := ctx.Value(serviceContextKey).(*ServiceContext)

	if !ok {
		fmt.Println("Missing tracing context, please use the original request context")
	}

	oq := sc.ObservationSequence(o.name)

	if sc.Debug {
	} else {
		if outBody, err := json.Marshal(value); err == nil {
			go func() {
				Log(Record{
					RequestContext:      sc.RequestContext,
					CauseContext:        sc.CauseContext,
					ExecutionContext:    sc.ExecutionContext,
					RecordType:          ObservedRecordType,
					Method:              "",
					Time:                time.Now(),
					Duration:            0,
					DepencencySequence:  0,
					ScopedSequence:      0,
					ObservationSequence: oq,
					ServiceName:         serviceName,
					ObservationName:     o.name,
					Host:                "",
					Uri:                 "",
					Header:              nil,
					Body:                outBody,
					StatusCode:          0,
				})
			}()
		} else {
			fmt.Printf("There was an error marshalling the observation. %s\n", err.Error())
		}
	}

	return value
}

func NewStateObserver[T any](name string) *StateObserver[T] {
	return &StateObserver[T]{
		name: name,
	}
}

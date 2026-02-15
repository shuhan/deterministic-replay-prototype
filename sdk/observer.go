package sdk

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"reflect"
	"time"
)

// Observer is an state observer that looks at and returns a state
// In debug mode it will injected and observation from recorded execution
type Observer[T any] interface {
	Observe(ctx context.Context, value T) T
}

type StateObserver[T any] struct {
	name   string
	encode func(T) ([]byte, error)
	decode func([]byte) (T, error)
}

func (o *StateObserver[T]) Observe(ctx context.Context, value T) (T, error) {
	sc, ok := ctx.Value(serviceContextKey).(*ServiceContext)

	if !ok {
		return value, fmt.Errorf("Missing tracing context, please use the original request context")
	}

	oq := sc.ObservationSequence(o.name)

	if sc.Debug {
		data := sc.ObservationData(o.name, oq)
		return o.Unmarshal(data)
	} else {
		if outBody, err := o.Marshal(value); err == nil {
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
			return value, err
		}
	}

	return value, nil
}

func (o *StateObserver[T]) Marshal(value T) ([]byte, error) {
	if o.encode == nil {
		return nil, fmt.Errorf("relevent encoder not found")
	}
	return o.encode(value)
}

func (o *StateObserver[T]) Unmarshal(data []byte) (T, error) {
	if o.decode == nil {
		var t T
		return t, fmt.Errorf("relevent decoder not found")
	}
	return o.decode(data)
}

func NewStateObserver[T any](name string) *StateObserver[T] {
	var value T
	vt := reflect.TypeOf(value)
	var encode func(T) ([]byte, error)
	var decode func([]byte) (T, error)

	switch vt.Kind() {
	case reflect.Bool,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128:
		size := vt.Size()
		encode = func(t T) ([]byte, error) {
			buffer := bytes.NewBuffer(make([]byte, size))
			buffer.Reset()
			err := binary.Write(buffer, binary.LittleEndian, t)
			return buffer.Bytes(), err
		}
		decode = func(b []byte) (T, error) {
			buffer := bytes.NewBuffer(b)
			var value T
			err := binary.Read(buffer, binary.LittleEndian, &value)
			return value, err
		}
	case reflect.String:
		encode = func(t T) ([]byte, error) {
			var v any = t
			str, _ := v.(string)
			return []byte(str), nil
		}
		decode = func(b []byte) (T, error) {
			var str any = string(b)
			t, _ := str.(T)
			return t, nil
		}
	case reflect.Int:
		encode = func(t T) ([]byte, error) {
			buffer := bytes.NewBuffer(make([]byte, 8))
			buffer.Reset()
			var tval any = t
			t32, _ := tval.(int)
			t64 := int64(t32)
			err := binary.Write(buffer, binary.LittleEndian, t64)
			return buffer.Bytes(), err
		}
		decode = func(b []byte) (T, error) {
			buffer := bytes.NewBuffer(b)
			var value int64
			err := binary.Read(buffer, binary.LittleEndian, &value)
			var v any = int(value)
			val, _ := v.(T)
			return val, err
		}
	case reflect.Uint:
		encode = func(t T) ([]byte, error) {
			buffer := bytes.NewBuffer(make([]byte, 8))
			buffer.Reset()
			var tval any = t
			t32, _ := tval.(uint)
			t64 := uint64(t32)
			err := binary.Write(buffer, binary.LittleEndian, t64)
			return buffer.Bytes(), err
		}
		decode = func(b []byte) (T, error) {
			buffer := bytes.NewBuffer(b)
			var value uint64
			err := binary.Read(buffer, binary.LittleEndian, &value)
			var v any = uint(value)
			val, _ := v.(T)
			return val, err
		}
	case reflect.Struct:
	case reflect.Array:
	case reflect.Slice:
	case reflect.Map:
	case reflect.Pointer:
	default:

	}

	return &StateObserver[T]{
		name:   name,
		encode: encode,
		decode: decode,
	}
}

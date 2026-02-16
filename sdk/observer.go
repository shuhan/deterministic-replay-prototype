package sdk

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
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

func (o *StateObserver[T]) Observe(ctx context.Context, value T) T {
	sc, ok := ctx.Value(serviceContextKey).(*ServiceContext)

	if !ok {
		fmt.Println("Missing tracing context, please use the original request context")
		return value
	}

	oq := sc.ObservationSequence()
	seq := sc.ObservationScopedDependencySequence(o.name)

	if sc.Debug {
		if data, ok := sc.ObservationData(o.name, seq); ok {
			val, err := o.Unmarshal(data.Body)
			if err != nil {
				fmt.Printf("Unmarshalling error for %s Error: %s\n", o.name, err.Error())
				return value
			}
			return val
		}
	} else {
		go func() {
			if outBody, err := o.Marshal(value); err == nil {
				Log(Record{
					RequestContext:      sc.RequestContext,
					CauseContext:        sc.CauseContext,
					ExecutionContext:    sc.ExecutionContext,
					RecordType:          ObservedRecordType,
					Method:              "",
					Time:                time.Now(),
					Duration:            0,
					DepencencySequence:  0,
					ScopedSequence:      seq,
					ObservationSequence: oq,
					ServiceName:         serviceName,
					ObservationName:     o.name,
					Host:                "",
					Uri:                 "",
					Header:              nil,
					Body:                outBody,
					StatusCode:          0,
				})
			} else {
				fmt.Printf("Error enocding observation: %s\n", err.Error())
			}
		}()
	}

	return value
}

func (o *StateObserver[T]) ObserveWithErr(ctx context.Context, value T) (T, error) {
	sc, ok := ctx.Value(serviceContextKey).(*ServiceContext)

	if !ok {
		return value, fmt.Errorf("Missing tracing context, please use the original request context")
	}

	oq := sc.ObservationSequence()
	seq := sc.ObservationScopedDependencySequence(o.name)

	if sc.Debug {
		if data, ok := sc.ObservationData(o.name, seq); ok {
			return o.Unmarshal(data.Body)
		}
	} else {
		go func() {
			if outBody, err := o.Marshal(value); err == nil {
				Log(Record{
					RequestContext:      sc.RequestContext,
					CauseContext:        sc.CauseContext,
					ExecutionContext:    sc.ExecutionContext,
					RecordType:          ObservedRecordType,
					Method:              "",
					Time:                time.Now(),
					Duration:            0,
					DepencencySequence:  0,
					ScopedSequence:      seq,
					ObservationSequence: oq,
					ServiceName:         serviceName,
					ObservationName:     o.name,
					Host:                "",
					Uri:                 "",
					Header:              nil,
					Body:                outBody,
					StatusCode:          0,
				})
			} else {
				fmt.Printf("Error enocding observation: %s\n", err.Error())
			}
		}()
	}

	return value, nil
}

func (o *StateObserver[T]) ObserveFunc(ctx context.Context, valueFunc func() T) T {
	sc, ok := ctx.Value(serviceContextKey).(*ServiceContext)

	if !ok {
		fmt.Println("Missing tracing context, please use the original request context")
		return valueFunc()
	}

	oq := sc.ObservationSequence()
	seq := sc.ObservationScopedDependencySequence(o.name)

	if sc.Debug {
		if data, ok := sc.ObservationData(o.name, seq); ok {
			val, err := o.Unmarshal(data.Body)
			if err != nil {
				fmt.Printf("Unmarshalling error for %s Error: %s\n", o.name, err.Error())
				return valueFunc()
			}
			return val
		}
		return valueFunc()
	} else {
		value := valueFunc()
		go func() {
			if outBody, err := o.Marshal(value); err == nil {
				Log(Record{
					RequestContext:      sc.RequestContext,
					CauseContext:        sc.CauseContext,
					ExecutionContext:    sc.ExecutionContext,
					RecordType:          ObservedRecordType,
					Method:              "",
					Time:                time.Now(),
					Duration:            0,
					DepencencySequence:  0,
					ScopedSequence:      seq,
					ObservationSequence: oq,
					ServiceName:         serviceName,
					ObservationName:     o.name,
					Host:                "",
					Uri:                 "",
					Header:              nil,
					Body:                outBody,
					StatusCode:          0,
				})
			} else {
				fmt.Printf("Error enocding observation: %s\n", err.Error())
			}
		}()

		return value
	}
}

func (o *StateObserver[T]) ObserveFuncWithErr(ctx context.Context, valueFunc func() (T, error)) (T, error) {
	sc, ok := ctx.Value(serviceContextKey).(*ServiceContext)

	if !ok {
		fmt.Println("Missing tracing context, please use the original request context")
		return valueFunc()
	}

	oq := sc.ObservationSequence()
	seq := sc.ObservationScopedDependencySequence(o.name)

	if sc.Debug {
		if data, ok := sc.ObservationData(o.name, seq); ok {

			if data.ObservationError != nil {
				val, _ := o.Unmarshal(data.Body)
				return val, fmt.Errorf("%s", string(data.ObservationError))
			}

			return o.Unmarshal(data.Body)
		}
		return valueFunc()
	} else {
		value, valueErr := valueFunc()
		go func() {
			if outBody, err := o.Marshal(value); err == nil {

				var errorBody []byte

				if valueErr != nil {
					errorBody = []byte(valueErr.Error())
				}

				Log(Record{
					RequestContext:      sc.RequestContext,
					CauseContext:        sc.CauseContext,
					ExecutionContext:    sc.ExecutionContext,
					RecordType:          ObservedRecordType,
					Method:              "",
					Time:                time.Now(),
					Duration:            0,
					DepencencySequence:  0,
					ScopedSequence:      seq,
					ObservationSequence: oq,
					ServiceName:         serviceName,
					ObservationName:     o.name,
					Host:                "",
					Uri:                 "",
					Header:              nil,
					Body:                outBody,
					ObservationError:    errorBody,
					StatusCode:          0,
				})
			} else {
				fmt.Printf("Error enocding observation: %s\n", err.Error())
			}
		}()

		return value, valueErr
	}
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
	encode, decode := getEncDec[T](vt)

	return &StateObserver[T]{
		name:   name,
		encode: encode,
		decode: decode,
	}
}

func getEncDec[T any](vt reflect.Type) (func(T) ([]byte, error), func([]byte) (T, error)) {
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
	case reflect.Struct, reflect.Array, reflect.Slice, reflect.Map, reflect.Pointer:
		size := vt.Size()
		encode = func(t T) ([]byte, error) {
			buffer := bytes.NewBuffer(make([]byte, 0, size))
			buffer.Reset()
			enc := gob.NewEncoder(buffer)
			err := enc.Encode(t)
			if err != nil {
				return nil, err
			}
			return buffer.Bytes(), nil
		}
		decode = func(b []byte) (T, error) {
			var value T
			dec := gob.NewDecoder(bytes.NewBuffer(b))
			err := dec.Decode(&value)
			return value, err
		}
	default:

	}

	return encode, decode
}

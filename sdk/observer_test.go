package sdk

import (
	"reflect"
	"testing"
)

func TestString(t *testing.T) {
	so := NewStateObserver[string]("Test")

	want := "Hello"
	data, err := so.Marshal(want)
	if err != nil {
		t.Error(err)
	}

	act, err := so.Unmarshal(data)
	if err != nil {
		t.Error(err)
	}

	if act != want {
		t.Errorf("Want %v Actual %v\n", want, act)
	}
}

func TestStringPtr(t *testing.T) {
	so := NewStateObserver[*string]("Test")

	want := "Hello"
	data, err := so.Marshal(&want)
	if err != nil {
		t.Error(err)
	}

	act, err := so.Unmarshal(data)
	if err != nil {
		t.Error(err)
	}

	if *act != want {
		t.Errorf("Want %v Actual %v\n", want, act)
	}
}

func TestBool(t *testing.T) {
	so := NewStateObserver[bool]("Test")

	want := true
	data, err := so.Marshal(want)
	if err != nil {
		t.Error(err)
	}

	act, err := so.Unmarshal(data)
	if err != nil {
		t.Error(err)
	}

	if act != want {
		t.Errorf("Want %v Actual %v\n", want, act)
	}
}

func TestUInt(t *testing.T) {
	so := NewStateObserver[uint]("Test")

	want := uint(13)
	data, err := so.Marshal(want)
	if err != nil {
		t.Error(err)
	}

	act, err := so.Unmarshal(data)
	if err != nil {
		t.Error(err)
	}

	if act != want {
		t.Errorf("Want %v Actual %v\n", want, act)
	}
}

func TestStruct(t *testing.T) {
	type Data struct {
		Name  string
		Age   int
		Roles []string
	}

	so := NewStateObserver[Data]("Test")

	want := Data{
		Name:  "John Doe",
		Age:   99,
		Roles: []string{"user", "creator"},
	}
	data, err := so.Marshal(want)
	if err != nil {
		t.Error(err)
	}

	act, err := so.Unmarshal(data)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(want, act) {
		t.Errorf("Want %v Actual %v\n", want, act)
	}
}

func TestArray(t *testing.T) {
	type Data struct {
		Name  string
		Age   int
		Roles []string
	}

	so := NewStateObserver[[2]Data]("Test")

	want := [2]Data{
		{
			Name:  "John Doe",
			Age:   99,
			Roles: []string{"user", "creator"},
		}, {
			Name:  "Jane Doe",
			Age:   89,
			Roles: []string{"admin", "creator"},
		},
	}
	data, err := so.Marshal(want)
	if err != nil {
		t.Error(err)
	}

	act, err := so.Unmarshal(data)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(want, act) {
		t.Errorf("Want %v Actual %v\n", want, act)
	}
}

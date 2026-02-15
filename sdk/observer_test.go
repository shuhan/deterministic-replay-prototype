package sdk

import (
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

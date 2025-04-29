package util

import (
	"strings"
	"testing"
)

type marshalTestType struct {
	Id          int    `koanf:"id"`
	Description string `koanf:"desc"`
}

func TestUtilMarshalToJsonBytes(t *testing.T) {
	o := marshalTestType{1, "test"}
	b := MarshalToJsonBytes(o)

	if b == nil {
		t.Errorf("failed to marshal koanf object to json, %v", o)
		return
	}

	val := string(b)
	if !strings.Contains(val, `"id":1`) ||
		!strings.Contains(val, `"desc":"test`) {
		t.Errorf("unexpected value marshaling struct to json, expected, %v, found %v", o, val)
	}
}

func TestUtilMarshalToYamlBytes(t *testing.T) {
	o := marshalTestType{1, "test"}
	b := MarshalToYamlBytes(o)

	if b == nil {
		t.Errorf("failed to marshal koanf object to yaml, %v", o)
		return
	}

	val := string(b)
	if !strings.Contains(val, `id: 1`) ||
		!strings.Contains(val, `desc: test`) {
		t.Errorf("unexpected value marshaling struct to yaml, expected, %v, found %v", o, val)
	}
}

// Copyright 2025 Metrostar Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

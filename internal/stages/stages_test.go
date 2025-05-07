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

package stages

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config/schema"
)

// TestLoadStagesHappy tests the successful loading of stages and compares
// the expected and actual results for correctness.
func TestLoadStagesHappy(t *testing.T) {
	expected := map[string]schema.StageConfig{
		"root": {
			Id:    "root",
			Order: 1,
		},
		"second": {
			Id:    "second",
			Order: 10,
		},
		"third": {
			Id:    "third",
			Order: 20,
		},
		"hasdependencies": {
			Id:    "hasdependencies",
			Order: 21,
		},
		"manual": {
			Id:    "manual",
			Order: 22,
		},
	}
	manual := NewStageConfig("manual")
	manual.Dependencies = []string{"root", "hasdependencies"}

	actual := LoadStages(map[string]schema.StageConfig{
		"manual": manual,
	}, filepath.Join("testdata", "TestLoadStagesHappy"))

	compare := func(l schema.StageConfig, r schema.StageConfig) bool {
		return l.Id == r.Id &&
			l.Order == r.Order
	}

	for ek, ev := range expected {
		av, ok := actual[ek]
		if !ok {
			t.Errorf("result map missing input key %s", ek)
		} else if !compare(ev, av) {
			t.Errorf("incorrect stage value for input key %s, expected %v, got %v", ek, ev, av)
		}
	}
}

// TestProcessStagePathHappy tests the successful processing of stage paths
// and compares the expected and actual results for correctness.
func TestProcessStagePathHappy(t *testing.T) {
	expected := map[string]schema.StageConfig{
		"default": {
			Id:          "default",
			Order:       10,
			Description: "default",
		},
		"hasconfig": {
			Id:          "hasconfig",
			Order:       -1,
			Description: "test stage with description",
		},
	}
	actual, err := processStagePath(filepath.Join("testdata", "TestProcessStagePathHappy"))
	if err != nil {
		t.Fatalf("unexpected error, %v", err)
	}

	compare := func(l schema.StageConfig, r schema.StageConfig) bool {
		return l.Id == r.Id &&
			l.Order == r.Order &&
			l.Description == r.Description
	}

	for ek, ev := range expected {
		av, ok := actual[ek]
		if !ok {
			t.Errorf("result map missing input key %s", ek)
		} else if !compare(ev, av) {
			t.Errorf("incorrect stage value for input key %s, expected %v, got %v", ek, ev, av)
		}
	}
}

func TestProcessStageDependenciesHappy(t *testing.T) {
	expected := map[string]int{
		"root":   100,
		"stage1": 101,
		"stage2": 100,
		"stage3": 102,
		"stage4": 103,
	}

	// assuming all stages have a default order value, derive
	// ordering based on explicitly defined dependencies
	stages := map[string]schema.StageConfig{
		"root": {
			Order: 100,
		},
		"stage1": {
			Order:        100,
			Dependencies: []string{"root"},
		},
		"stage2": {
			Order: 100,
		},
		"stage3": {
			Order:        100,
			Dependencies: []string{"stage1", "stage2"},
		},
		"stage4": {
			Order:        100,
			Dependencies: []string{"stage2", "stage3"},
		},
	}

	actual, err := processStageDependencies(stages)
	if err != nil {
		t.Fatalf("unexpected error, %v", err)
	}

	for ek, ev := range expected {
		av, ok := actual[ek]
		if !ok {
			t.Errorf("result map missing input key %s", ek)
		} else if ev != av.Order {
			t.Errorf("incorrect stage order value for input key %s, expected %d, got %d", ek, ev, av.Order)
		}
	}
}

func TestProcessStageDependenciesCircularError(t *testing.T) {
	stages := map[string]schema.StageConfig{
		"stage1": {
			Dependencies: []string{"stage2"},
		},
		"stage2": {
			Dependencies: []string{"stage1"},
		},
	}

	_, err := processStageDependencies(stages)
	if err == nil {
		t.Error("didn't get an error as expected")
	} else if !strings.Contains(err.Error(), "max iterations reached") {
		t.Errorf("unexpected error, expected %s, got %v", "max iterations reached", err)
	}
}

func TestParseStageDirNameHappy(t *testing.T) {
	dir := "101-foobar"
	order, id, err := parseStageDirName(dir)

	if err != nil {
		t.Errorf("error parsing stage directory %s, %v", dir, err)
	}

	if order != 101 {
		t.Errorf("failed to parse order value of valid directory name, expected %d, got %d", 101, order)
	}

	if id != "foobar" {
		t.Errorf("failed to parse id value of valid directory name, expected %s, got %s", "foobar", id)
	}
}

func TestParseStageDirNameMissingOrder(t *testing.T) {
	dir := "foobar"
	order, id, err := parseStageDirName(dir)

	// expecting an err in this case
	if err == nil {
		t.Error("didn't get an error as expected")
	} else if !strings.Contains(err.Error(), "unsupported stage name format") {
		t.Errorf("unexpected error, expected %s, got %v", "unsupported stage name format", err)
	}

	if order != -1 {
		t.Errorf("failed to parse order value of valid directory name, expected %d, got %d", -1, order)
	}

	if id != "foobar" {
		t.Errorf("failed to parse id value of valid directory name, expected %s, got %s", "foobar", id)
	}
}

func TestParseStageDirNameNonnumericOrder(t *testing.T) {
	dir := "bad-foobar"
	order, id, err := parseStageDirName(dir)

	// expecting an err in this case
	if err == nil {
		t.Error("didn't get an error as expected")
	} else if !strings.Contains(err.Error(), "invalid stage name prefix") {
		t.Errorf("unexpected error, expected %s, got %v", "invalid stage name prefix", err)
	}

	if order != -1 {
		t.Errorf("failed to parse order value of valid directory name, expected %d, got %d", -1, order)
	}

	if id != "foobar" {
		t.Errorf("failed to parse id value of valid directory name, expected %s, got %s", "foobar", id)
	}
}

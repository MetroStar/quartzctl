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

package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config/schema"
)

type TestProviderCheckResult struct {
	headers []string
	rows    []ProviderCheckResultRow
}

func (r TestProviderCheckResult) ToTable() ([]string, []ProviderCheckResultRow) {
	return r.headers, r.rows
}

func TestProviderCheck(t *testing.T) {
	opts := NewProviderCheckOpts(context.Background(), ProviderFactory{
		cfg: schema.QuartzConfig{
			Name: "testcluster",
			Providers: schema.ProvidersConfig{
				Cloud: "local",
			},
			Aws: schema.AwsConfig{
				Region: "local",
			},
			Mirror: schema.MirrorConfig{
				ImageRepository: schema.MirrorImageRepositoryConfig{
					Enabled: true,
				},
			},
		},
		dnsProviderClient: NewEmptyProvider("testdns", fmt.Errorf("testing")),
	})

	// TODO: have this return something or take a callback so we can make assertions
	Check(context.Background(), &opts)
}

func TestProviderCheckPrintTableEmpty(t *testing.T) {
	name := "test"
	res := TestProviderCheckResult{
		headers: []string{},
		rows:    []ProviderCheckResultRow{},
	}

	printTable(name, res)
}

func TestProviderCheckPrintTablePadded(t *testing.T) {
	name := "test"
	res := TestProviderCheckResult{
		headers: []string{"col1", "col2", "col3"},
		rows: []ProviderCheckResultRow{
			{Data: []string{"cell1-1", "cell1-2", "cell1-3"}, Error: fmt.Errorf("test error 1"), Status: false}, // error
			{Data: []string{"cell2-1", "cell2-2"}, Error: fmt.Errorf("test error 2"), Status: true},             // warning
			{Data: []string{"cell3"}, Status: true},                                                             // ok
		},
	}

	printTable(name, res)
}

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
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"
)

var (
	lock = &sync.Mutex{}
)

// IProviderCheckResult defines the interface for provider check results.
type ProviderCheckResult interface {
	// ToTable converts the check result into table headers and rows.
	ToTable() ([]string, []ProviderCheckResultRow)
}

// ProviderCheckResultRow represents a single row in the provider check result table.
type ProviderCheckResultRow struct {
	Status bool     // Status indicates whether the check was successful.
	Data   []string // Data contains the row's data fields.
	Error  error    // Error contains any error associated with the row.
}

// ProviderCheckOpts contains options for performing provider checks.
type ProviderCheckOpts struct {
	checks []Provider // checks is the list of providers to check.
}

// NewProviderCheckOpts creates a new ProviderCheckOpts instance.
// It initializes the list of providers to check by iterating over the provided factory.
func NewProviderCheckOpts(ctx context.Context, f ProviderFactory) ProviderCheckOpts {
	var checks []Provider

	// NOTE: the Kubernetes provider is not included in the checks
	// as this function is primarily used to check provider configurations
	// before the platform is installed, making the Kubernetes check irrelevant
	m := map[string]func(ctx context.Context) (Provider, error){
		"Cloud":         func(ctx context.Context) (Provider, error) { return f.Cloud(ctx) },
		"Dns":           func(ctx context.Context) (Provider, error) { return f.Dns(ctx) },
		"SourceControl": f.SourceControl,
		"ImageRegistry": f.ImageRegistry,
	}

	for name, fn := range m {
		p, err := fn(ctx)

		// don't add the same provider twice
		if slices.ContainsFunc(checks, func(ip Provider) bool {
			return p != nil && reflect.TypeOf(ip) == reflect.TypeOf(p)
		}) {
			continue
		}

		if p != nil {
			checks = append(checks, p)
		} else if err != nil {
			checks = append(checks, NewEmptyProvider(name, err))
		}
	}

	return ProviderCheckOpts{
		checks: checks,
	}
}

// Check performs access checks for all providers in the given options.
// It logs the results and execution statistics.
func Check(ctx context.Context, opts *ProviderCheckOpts) {
	start := time.Now()

	wg := sync.WaitGroup{}
	wg.Add(len(opts.checks))

	for _, c := range opts.checks {
		go func(ic Provider) {
			defer wg.Done()
			res := ic.CheckAccess(ctx)
			printTable(ic.ProviderName(), res)
		}(c)
	}

	wg.Wait()

	log.Debug("Check stats", "start", start, "duration", time.Since(start))
}

// printTable formats and prints the provider check results as a table.
// It synchronizes output to avoid interleaving with other logs.
func printTable(providerName string, r ProviderCheckResult) {
	headers, rows := r.ToTable()

	log.Debug("Formatting table", "headers", headers, "rows", rows)

	if headers == nil {
		headers = []string{}
	}

	if len(rows) == 0 {
		rows = []ProviderCheckResultRow{
			{Status: false, Error: fmt.Errorf("no data")},
		}
	}

	for _, v := range rows {
		if v.Error != nil {
			headers = append(headers, "Error")
			break
		}
	}

	var rs [][]string
	colsLen := len(headers)

	for _, v := range rows {
		row := slices.Clone(v.Data)

		// padding up to the number of columns needed
		dataLen := len(v.Data)
		for range colsLen - dataLen {
			row = append(row, "")
		}

		// insert error in last column
		if v.Error != nil {
			row[len(row)-1] = util.TruncateStringE(v.Error.Error(), 20)
		}

		rs = append(rs, row)
	}

	// synchronize writing so the header and table don't get split unexpectedly
	lock.Lock()
	defer lock.Unlock()

	util.Msg(providerName)
	util.PrintRowStatusTable(headers, rs, func(i int, row []string) util.RowStatus {
		v := rows[i]
		if !v.Status {
			return util.StatusError
		} else if v.Error != nil {
			return util.StatusWarning
		}

		return util.StatusOk
	})
}

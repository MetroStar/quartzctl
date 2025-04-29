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

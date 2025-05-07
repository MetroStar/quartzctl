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
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/MetroStar/quartzctl/internal/util"
)

func TestProviderCloudflareClientProviderName(t *testing.T) {
	_, err := NewCloudflareClient(nil, "", "", "", "")
	if err == nil {
		t.Error("expected error from cloudflare client for missing required arguments")
	}

	c, err := NewCloudflareClient(nil, "", "12345", "test", "example.com")
	if err != nil {
		t.Errorf("unexpected error from cloudflare client constructor, %v", err)
	}

	if c.ProviderName() != "Cloudflare" {
		t.Errorf("unexpected default provider name from cloudflare client, expected %v, found %v", "Cloudflare", c.ProviderName())
	}
}

func TestProviderCloudflareClientCheckAccess(t *testing.T) {
	httpClient := util.HttpClientFactoryMock{
		Callback: func(req *http.Request) *http.Response {
			if req.Method != "GET" {
				t.Errorf("unexpected http request method, expected %v, found %v", "GET", req.Method)
			}

			if req.URL.Host != "api.cloudflare.com" &&
				!strings.Contains(req.URL.String(), "example.com") {
				t.Errorf("unexpected http request url, expected %v, found %v", "GET", req.URL.String())
			}

			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
				{
					"Success": true,
					"Errors": [],
					"Messages": [],
					"Result": [{
						"Id": "1",
						"Name":	"testzone",
						"Permissions": ["#zone:read", "#dns_records:edit", "#dns_records:read"]
					}]
				}
				`)),
				Header: make(http.Header),
			}
		},
	}

	c, err := NewCloudflareClient(httpClient, "", "12345", "test", "example.com")
	if err != nil {
		t.Errorf("unexpected error from cloudflare client constructor, %v", err)
	}

	res := c.CheckAccess(context.Background())
	switch r := res.(type) {
	case CloudflareAccessCheckResult:
		if !r.Status ||
			r.Error != nil {
			t.Errorf("unexpected response value from cloudflare check access, %v", r)
		}
	default:
		t.Errorf("unexpected response type from cloudflare check access, %v", r)
	}

	headers, rows := res.ToTable()
	if len(rows) != 1 {
		t.Errorf("unexpected response from cloudclare check access table, %v, %v", headers, rows)
	}
}

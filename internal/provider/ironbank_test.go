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
	"testing"

	"github.com/MetroStar/quartzctl/internal/util"
)

func TestProviderIronbankClientProviderName(t *testing.T) {
	_, err := NewIronbankClient(nil, "", "", "")
	if err == nil {
		t.Error("expected error from ironbank client for missing required arguments")
	}

	c, err := NewIronbankClient(nil, "", "testuser", "supersecretpassword")
	if err != nil {
		t.Errorf("unexpected error from ironbank client constructor, %v", err)
	}

	if c.ProviderName() != "Ironbank" {
		t.Errorf("unexpected default provider name from ironbank client, expected %v, found %v", "Ironbank", c.ProviderName())
	}
}

func TestProviderIronbankClientCheckAccess(t *testing.T) {
	httpClient := util.HttpClientFactoryMock{
		Callback: func(req *http.Request) *http.Response {
			if req.Method != "GET" {
				t.Errorf("unexpected http request method, expected %v, found %v", "GET", req.Method)
			}

			if req.URL.Host != "registry1.dso.mil" {
				t.Errorf("unexpected http request url, expected %v, found %v", "GET", req.URL.String())
			}

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
				Header:     make(http.Header),
			}
		},
	}

	c, err := NewIronbankClient(httpClient, "", "testuser", "supersecretpassword")
	if err != nil {
		t.Errorf("unexpected error from ironbank client constructor, %v", err)
	}

	res := c.CheckAccess(context.Background())
	switch r := res.(type) {
	case IronbankCheckAccessResult:
		if r.StatusCode != 200 ||
			r.Username != "testuser" ||
			r.Error != nil {
			t.Errorf("unexpected response value from ironbank check access, %v", r)
		}
	default:
		t.Errorf("unexpected response type from ironbank check access, %v", r)
	}

	headers, rows := res.ToTable()
	if len(rows) != 1 {
		t.Errorf("unexpected response from ironbank check access table, %v, %v", headers, rows)
	}
}

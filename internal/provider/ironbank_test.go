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

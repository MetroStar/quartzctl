package stages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config/schema"
)

func TestHttpStageCheckRunHappyNoContent(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	sut := HttpStageCheck{
		Url: svr.URL,
	}
	err := sut.Run(context.TODO(), schema.QuartzConfig{})
	if err != nil {
		t.Errorf("Unexpected error in http check (empty), %v", err)
	}
}

func TestHttpStageCheckRunHappyLiteral(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("this is a test, and it worked"))
	}))
	defer svr.Close()

	sut := HttpStageCheck{
		Url: svr.URL,
		Content: schema.StageChecksHttpContentConfig{
			Value: "this is a test, and it worked",
		},
	}
	err := sut.Run(context.TODO(), schema.QuartzConfig{})
	if err != nil {
		t.Errorf("Unexpected error in http check (literal), %v", err)
	}
}

func TestHttpStageCheckRunHappyJson(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{\"key1\":\"value1\"}"))
	}))
	defer svr.Close()

	sut := HttpStageCheck{
		Url: svr.URL,
		Content: schema.StageChecksHttpContentConfig{
			Value: "value1",
			Json: schema.StageChecksHttpJsonContentConfig{
				Key: "key1",
			},
		},
	}
	err := sut.Run(context.TODO(), schema.QuartzConfig{})
	if err != nil {
		t.Errorf("Unexpected error in http check (json), %v", err)
	}
}

func TestHttpStageCheckRunNoMatch(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{\"key1\":\"value1\"}"))
	}))
	defer svr.Close()

	sut := HttpStageCheck{
		Url: svr.URL,
		Content: schema.StageChecksHttpContentConfig{
			Value: "value2",
			Json: schema.StageChecksHttpJsonContentConfig{
				Key: "key1",
			},
		},
	}
	err := sut.Run(context.TODO(), schema.QuartzConfig{})
	if err == nil {
		t.Error("Expected error in http check (json) not found")
	}
}

func TestHttpStageCheckFormatUrl(t *testing.T) {
	cfg := schema.QuartzConfig{
		Dns: schema.DnsConfig{
			Domain: "example.com",
		},
	}

	u1 := HttpStageCheck{App: "foobar"}.formatUrl(cfg)
	if u1 != "https://foobar.example.com" {
		t.Errorf("invalid response, expected %s, found %s", "https://foobar.example.com", u1)
	}

	u2 := HttpStageCheck{App: "keycloak"}.formatUrl(cfg)
	if u2 != "https://keycloak.auth.example.com" {
		t.Errorf("invalid response, expected %s, found %s", "https://keycloak.auth.example.com", u2)
	}
}

func TestHttpStageCheckRunResponseError(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer svr.Close()

	sut := HttpStageCheck{Url: svr.URL, StatusCodes: []int{200}}
	err := sut.Run(context.TODO(), schema.QuartzConfig{})
	if err == nil {
		t.Error("Didn't get expected error in http check")
	}
}

func TestHttpStageCheckRunProtocolError(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer svr.Close()

	sut := HttpStageCheck{Url: "definitelynotavalidurl"}
	err := sut.Run(context.TODO(), schema.QuartzConfig{})
	if err == nil {
		t.Error("Didn't get expected error in http check")
	}
}

func TestHttpStageCheckId(t *testing.T) {
	id1 := HttpStageCheck{Url: "example.com"}.Id()
	if id1 != "example.com" {
		t.Errorf("invalid id (url), expected %s, found %s", "example.com", id1)
	}

	id2 := HttpStageCheck{Path: "/foobar"}.Id()
	if id2 != "/foobar" {
		t.Errorf("invalid id (path), expected %s, found %s", "/foobar", id2)
	}
}

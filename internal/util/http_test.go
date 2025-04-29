package util

import "testing"

func TestUtilHttpNewClient(t *testing.T) {
	f := NewHttpClientFactory()
	c := f.NewClient()
	if c == nil {
		t.Error("unexpected response from http client ctor")
	}
}

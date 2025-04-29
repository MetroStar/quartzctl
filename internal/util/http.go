package util

import (
	"net/http"
)

// IHttpClientFactory defines an interface for creating new HTTP clients.
type HttpClientFactory interface {
	NewClient() *http.Client
}

// HttpClientFactory is a default implementation of IHttpClientFactory
// that creates standard HTTP clients.
type HttpClientFactoryImpl struct{}

func NewHttpClientFactory() HttpClientFactory {
	return HttpClientFactoryImpl{}
}

// NewClient creates and returns a new instance of an HTTP client.
func (f HttpClientFactoryImpl) NewClient() *http.Client {
	return &http.Client{}
}

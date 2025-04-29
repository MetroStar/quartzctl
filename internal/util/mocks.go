package util

import "net/http"

// HttpClientFactoryMock is a mock implementation of IHttpClientFactory
// for testing purposes. It uses a custom RoundTripFunc to handle HTTP requests.
type HttpClientFactoryMock struct {
	Callback RoundTripFunc
}

// RoundTripFunc defines a function type that processes HTTP requests
// and returns HTTP responses. It is used for mocking HTTP client behavior.
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip executes the RoundTripFunc for the given HTTP request
// and returns the corresponding HTTP response.
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewClient creates and returns a new instance of an HTTP client
// with a custom transport that uses the provided RoundTripFunc.
func (f HttpClientFactoryMock) NewClient() *http.Client {
	return &http.Client{
		Transport: f.Callback,
	}
}

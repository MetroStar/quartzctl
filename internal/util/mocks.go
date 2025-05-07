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

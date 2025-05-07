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

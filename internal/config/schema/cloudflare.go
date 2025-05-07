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

package schema

// CloudflareCredentials represents the credentials for accessing Cloudflare.
type CloudflareCredentials struct {
	AccountId string `koanf:"account_id"` // The Cloudflare account ID.
	ApiToken  string `koanf:"api_token"`  // The API token for Cloudflare.
	Email     string `koanf:"email"`      // The email associated with the Cloudflare account.
}

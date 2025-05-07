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

// AlertsConfig represents the configuration for alerts.
type AlertsConfig struct {
	Subscriptions []AlertsSubscriptionsConfig `koanf:"subscriptions"` // A list of alert subscriptions.
}

// AlertsSubscriptionsConfig represents the configuration for an individual alert subscription.
type AlertsSubscriptionsConfig struct {
	Protocol string `koanf:"protocol"` // The protocol used for the subscription (e.g., "email", "sms").
	Endpoint string `koanf:"endpoint"` // The endpoint for the subscription (e.g., email address or phone number).
}

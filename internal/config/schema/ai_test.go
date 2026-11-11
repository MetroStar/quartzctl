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

import "testing"

func TestDefaultAIConfig(t *testing.T) {
	cfg := DefaultAIConfig()
	if cfg.Provider != "bedrock" {
		t.Fatalf("provider = %q, want bedrock", cfg.Provider)
	}
	if cfg.Bedrock.InferenceProfile == "" || cfg.Bedrock.FoundationModel == "" {
		t.Fatal("default Bedrock profile and foundation model must be configured")
	}
	if len(cfg.Bedrock.DestinationRegions) == 0 {
		t.Fatal("default Bedrock destination regions must be configured")
	}
}

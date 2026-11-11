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

type AIConfig struct {
	Provider string        `koanf:"provider"`
	Bedrock  BedrockConfig `koanf:"bedrock"`
}

type BedrockConfig struct {
	InferenceProfile   string   `koanf:"inference_profile"`
	FoundationModel    string   `koanf:"foundation_model"`
	Region             string   `koanf:"region"`
	DestinationRegions []string `koanf:"destination_regions"`
}

func DefaultAIConfig() AIConfig {
	return AIConfig{
		Provider: "bedrock",
		Bedrock: BedrockConfig{
			InferenceProfile:   "us.openai.gpt-5.6-terra",
			FoundationModel:    "openai.gpt-5.6-terra",
			Region:             "",
			DestinationRegions: []string{"us-east-1", "us-east-2", "us-west-2"},
		},
	}
}

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

package provider

import (
	"context"
	"testing"
)

func TestProviderLocalClient(t *testing.T) {
	c := LocalClient{Name: "test"}

	name := c.ProviderName()
	cfgRes := c.CheckConfig()
	accRes := c.CheckAccess(context.Background())
	id, _ := c.CurrentIdentity(context.Background())

	// for coverage
	c.StateBackendInfo("")
	c.CreateStateBackend(context.Background())
	c.DestroyStateBackend(context.Background())
	c.KubeconfigInfo(context.Background())
	c.PrintConfig()
	c.PrintClusterInfo(context.Background())
	c.PrepareAccount(context.Background())

	if name != "Local" ||
		cfgRes != nil ||
		accRes == nil ||
		id.AccountId != "local" {
		t.Errorf("unexpected result from local client provider")
	}
}

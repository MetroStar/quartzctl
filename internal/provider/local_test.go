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

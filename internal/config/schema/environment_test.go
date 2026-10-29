package schema

import "testing"

func TestQuartzEvaluatorInfrastructureDefaults(t *testing.T) {
	config := NewInfrastructureEnvironmentConfig("infra", "Infrastructure")
	if _, exists := config.Applications["epyon"]; exists {
		t.Fatal("fresh-install defaults must not provision the legacy Epyon client")
	}
	app, exists := config.Applications["quartz-evaluator"]
	if !exists {
		t.Fatal("Quartz Evaluator infrastructure application is missing")
	}
	if app.Description != "Quartz Evaluator" || app.Public || app.Disabled {
		t.Fatalf("unexpected evaluator identity or SSO settings: %+v", app)
	}
	if len(app.CallbackUrls) != 1 || app.CallbackUrls[0].Path != "/oauth2/callback" {
		t.Fatalf("unexpected oauth2-proxy callbacks: %+v", app.CallbackUrls)
	}
	if app.Lookup.Ingress.Namespace != "quartz-evaluator" || app.Lookup.Ingress.Name != "quartz-evaluator" {
		t.Fatalf("lookup does not match the renamed chart: %+v", app.Lookup)
	}
}

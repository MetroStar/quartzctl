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

package schema

// AuthConfig represents the authentication configuration, including service accounts, users, and groups.
type AuthConfig struct {
	ServiceAccount AuthServiceAccountConfig   `koanf:"service_account"` // Configuration for the service account.
	Users          map[string]AuthUserConfig  `koanf:"users"`           // Configuration for individual users.
	Groups         map[string]AuthGroupConfig `koanf:"groups"`          // Configuration for user groups.
}

// DefaultAuthConfig returns the default authentication configuration.
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		ServiceAccount: AuthServiceAccountConfig{
			Enabled:           false,
			Name:              "quartz-cli-admin",
			Namespace:         "kube-system",
			ExpirationSeconds: int64(60 * 60 * 3), // 3 hrs
		},
		Users: map[string]AuthUserConfig{
			"quartzadmin": {
				Password: AuthUserPasswordConfig{
					Value:     "ChangeMe1234!",
					Temporary: true,
				},
				Groups:       []string{"administrators", "readers"},
				Environments: []string{"infra"},
			},
			"quartzuser": {
				Password: AuthUserPasswordConfig{
					Value:     "ChangeMe1234!",
					Temporary: true,
				},
				Groups:       []string{"readers"},
				Environments: []string{"infra"},
			},
		},
		Groups: map[string]AuthGroupConfig{
			"administrators": {
				Environments: []string{"infra"},
			},
			"readers": {
				Environments: []string{"infra"},
			},
		},
	}
}

// AuthServiceAccountConfig represents the configuration for a Kubernetes service account.
type AuthServiceAccountConfig struct {
	Enabled           bool   `koanf:"enabled"`            // Indicates if the service account is enabled.
	Name              string `koanf:"name"`               // The name of the service account.
	Namespace         string `koanf:"namespace"`          // The namespace of the service account.
	ExpirationSeconds int64  `koanf:"expiration_seconds"` // The expiration time for the service account token in seconds.
}

// AuthUserConfig represents the configuration for an individual user.
type AuthUserConfig struct {
	Disabled     bool                   `koanf:"disabled"`      // Indicates if the user is disabled.
	FirstName    string                 `koanf:"first_name"`    // The first name of the user.
	LastName     string                 `koanf:"last_name"`     // The last name of the user.
	EmailAddress string                 `koanf:"email_address"` // The email address of the user.
	Password     AuthUserPasswordConfig `koanf:"password"`      // The password configuration for the user.
	Groups       []string               `koanf:"groups"`        // The groups the user belongs to.
	Environments []string               `koanf:"environments"`  // The environments the user has access to.
	Test         bool                   `koanf:"test"`          // Indicates if the user is a test user.
	Count        int                    `koanf:"count"`         // The number of users to create (for bulk creation).
}

// AuthUserPasswordConfig represents the configuration for a user's password.
type AuthUserPasswordConfig struct {
	Temporary bool   `koanf:"temporary"` // Indicates if the password is temporary.
	Value     string `koanf:"value"`     // The value of the password.
}

// AuthGroupConfig represents the configuration for a user group.
type AuthGroupConfig struct {
	Disabled     bool     `koanf:"disabled"`     // Indicates if the group is disabled.
	Roles        []string `koanf:"roles"`        // The roles assigned to the group.
	Environments []string `koanf:"environments"` // The environments the group has access to.
}

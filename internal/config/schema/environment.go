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

// InfrastructureEnvironmentConfig represents the configuration for an infrastructure environment.
type InfrastructureEnvironmentConfig struct {
	Name                string                                     `koanf:"name"`
	Description         string                                     `koanf:"description"`
	Type                string                                     `koanf:"type"`
	RegistrationAllowed bool                                       `koanf:"registration_allowed"`
	CustomThemeEnabled  bool                                       `koanf:"custom_theme_enabled"`
	Otp                 ApplicationEnvironmentOtpConfig            `koanf:"otp"`
	Applications        map[string]InfrastructureApplicationConfig `koanf:"applications"`
}

// InfrastructureApplicationConfig represents the configuration for an infrastructure application.
type InfrastructureApplicationConfig struct {
	Disabled                   bool                              `koanf:"disabled"`
	Description                string                            `koanf:"description"`
	BaseUrl                    string                            `koanf:"base_url"`
	CallbackUrls               []ApplicationCallbackConfig       `koanf:"callback_urls"`
	Db                         InfrastructureApplicationDbConfig `koanf:"db"`
	DefaultPath                string                            `koanf:"default_path"`
	Scopes                     []string                          `koanf:"scopes"`
	AccessTokenLifespanSeconds int                               `koanf:"access_token_lifespan_seconds"`
	Keycloak                   map[string]interface{}            `koanf:"keycloak"`
	Lookup                     ApplicationLookupConfig           `koanf:"lookup"`
}

type InfrastructureApplicationDbConfig struct {
	Enabled  bool   `koanf:"enabled"`
	Admin    bool   `koanf:"admin"`
	Username string `koanf:"username"`
	DbName   string `koanf:"db_name"`
}

// ApplicationEnvironmentConfig represents the configuration for an application environment.
type ApplicationEnvironmentConfig struct {
	Name                string                          `koanf:"name"`
	Description         string                          `koanf:"description"`
	Next                string                          `koanf:"next"`
	Type                string                          `koanf:"type"`
	RegistrationAllowed bool                            `koanf:"registration_allowed"`
	CustomThemeEnabled  bool                            `koanf:"custom_theme_enabled"`
	Otp                 ApplicationEnvironmentOtpConfig `koanf:"otp"`
	Enabled             bool                            `koanf:"enabled"`
	Keycloak            map[string]interface{}          `koanf:"keycloak"`
}

type ApplicationEnvironmentOtpConfig struct {
	Enabled  bool `koanf:"enabled"`
	Required bool `koanf:"required"`
}

// ApplicationLookupConfig represents the configuration for looking up application resources.
type ApplicationLookupConfig struct {
	Enabled          bool                               `koanf:"enabled"`
	AdminCredentials ApplicationLookupCredentialsConfig `koanf:"admin_credentials"`
	Ingress          ApplicationLookupIngressConfig     `koanf:"ingress"`
}

type ApplicationLookupCredentialsConfig struct {
	Username string                                   `koanf:"username"`
	Secret   ApplicationLookupCredentialsSecretConfig `koanf:"secret"`
}

type ApplicationLookupCredentialsSecretConfig struct {
	Name        string `koanf:"name"`
	Namespace   string `koanf:"namespace"`
	UsernameKey string `koanf:"username_key"`
	PasswordKey string `koanf:"password_key"`
}

type ApplicationLookupIngressConfig struct {
	Name      string `koanf:"name"`
	Namespace string `koanf:"namespace"`
	Kind      string `koanf:"kind"`
	Group     string `koanf:"group"`
	Version   string `koanf:"version"`
}

// DefaultApplicationEnvironments returns the default application environments.
func DefaultApplicationEnvironments() map[string]ApplicationEnvironmentConfig {
	return map[string]ApplicationEnvironmentConfig{
		"dev":   NewApplicationEnvironmentConfig("dev", "Quartz (DEV)", "stage"),
		"stage": NewApplicationEnvironmentConfig("stage", "Quartz (STAGE)", "prod"),
		"prod":  NewApplicationEnvironmentConfig("prod", "Quartz", ""),
	}
}

// NewApplicationEnvironmentConfig creates a new ApplicationEnvironmentConfig with default values.
func NewApplicationEnvironmentConfig(name string, desc string, next string) ApplicationEnvironmentConfig {
	return ApplicationEnvironmentConfig{
		Enabled:             true,
		Name:                name,
		Description:         desc,
		Type:                "app",
		Next:                next, // next environment in promotion sequence (ex. dev -> stage -> prod)
		RegistrationAllowed: false,
		CustomThemeEnabled:  false,
		Otp: ApplicationEnvironmentOtpConfig{
			Enabled:  false,
			Required: false,
		},
		Keycloak: map[string]interface{}{
			"mappers": map[string]interface{}{
				"groups": map[string]interface{}{
					"protocol":       "openid-connect",
					"protocolMapper": "oidc-group-membership-mapper",
					"config": map[string]string{
						"full.path":                 "false",
						"id.token.claim":            "true",
						"access.token.claim":        "true",
						"claim.name":                "groups",
						"userinfo.token.claim":      "true",
						"introspection.token.claim": "true",
						"multivalued":               "true",
					},
				},
			},
		},
	}
}

// NewInfrastructureEnvironmentConfig creates a new InfrastructureEnvironmentConfig with default values.
func NewInfrastructureEnvironmentConfig(name string, desc string) InfrastructureEnvironmentConfig {
	return InfrastructureEnvironmentConfig{
		Name:                name,
		Description:         desc,
		Type:                "infra",
		RegistrationAllowed: false,
		CustomThemeEnabled:  false,
		Otp: ApplicationEnvironmentOtpConfig{
			Enabled:  false,
			Required: false,
		},
		// TODO: extend core application enabling to yaml config?
		Applications: map[string]InfrastructureApplicationConfig{
			"kiali": {
				Description:  "Kiali",
				CallbackUrls: []ApplicationCallbackConfig{{Path: "/kiali"}},
				Lookup:       NewApplicationLookupConfig("kiali", "", "", "", "", "kiali"),
			},
			"argocd": {
				Description: "ArgoCD",
				CallbackUrls: []ApplicationCallbackConfig{
					{Path: "/auth/callback"},
					{Path: "/api/dex/callback"},
				},
				DefaultPath:                "/applications",
				Scopes:                     []string{"argocd"},
				AccessTokenLifespanSeconds: 60 * 60 * 12, // 12 hrs
				Lookup:                     NewApplicationLookupConfig("argocd", "argocd-initial-admin-secret", "admin", "", "password", "argocd-argocd"),
				Keycloak: map[string]interface{}{
					"mappers": map[string]interface{}{
						"username": map[string]interface{}{
							"protocol":       "openid-connect",
							"protocolMapper": "oidc-usermodel-property-mapper",
							"config": map[string]string{
								"user.attribute":            "username",
								"claim.name":                "preferred_username",
								"jsonType.label":            "String",
								"id.token.claim":            "true",
								"access.token.claim":        "true",
								"userinfo.token.claim":      "true",
								"introspection.token.claim": "true",
							},
						},
						"groups": map[string]interface{}{
							"protocol":       "openid-connect",
							"protocolMapper": "oidc-group-membership-mapper",
							"config": map[string]string{
								"claim.name":                "groups",
								"full.path":                 "true",
								"multivalued":               "true",
								"id.token.claim":            "true",
								"access.token.claim":        "true",
								"userinfo.token.claim":      "true",
								"introspection.token.claim": "true",
							},
						},
						"fullname": map[string]interface{}{
							"protocol":       "openid-connect",
							"protocolMapper": "oidc-full-name-mapper",
							"config": map[string]string{
								"id.token.claim":            "true",
								"access.token.claim":        "true",
								"userinfo.token.claim":      "true",
								"introspection.token.claim": "true",
							},
						},
						"nickname": map[string]interface{}{
							"protocol":       "openid-connect",
							"protocolMapper": "oidc-usermodel-attribute-mapper",
							"config": map[string]string{
								"user.attribute":            "nickname",
								"claim.name":                "nickname",
								"jsonType.label":            "String",
								"id.token.claim":            "true",
								"access.token.claim":        "true",
								"userinfo.token.claim":      "true",
								"introspection.token.claim": "true",
							},
						},
						"email": map[string]interface{}{
							"protocol":       "openid-connect",
							"protocolMapper": "oidc-usermodel-property-mapper",
							"config": map[string]string{
								"user.attribute":            "email",
								"claim.name":                "email",
								"jsonType.label":            "String",
								"id.token.claim":            "true",
								"access.token.claim":        "true",
								"userinfo.token.claim":      "true",
								"introspection.token.claim": "true",
							},
						},
						"profile": map[string]interface{}{
							"protocol":       "openid-connect",
							"protocolMapper": "oidc-usermodel-attribute-mapper",
							"config": map[string]string{
								"user.attribute":            "profile",
								"claim.name":                "profile",
								"jsonType.label":            "String",
								"id.token.claim":            "true",
								"access.token.claim":        "true",
								"userinfo.token.claim":      "true",
								"introspection.token.claim": "true",
							},
						},
					},
				},
			},
			"sonarqube": {
				Description: "SonarQube",
				CallbackUrls: []ApplicationCallbackConfig{
					{Path: "/oauth2/callback/oidc"},
				},
				Db: InfrastructureApplicationDbConfig{
					Enabled: true,
					Admin:   true,
				},
				Lookup: NewApplicationLookupConfig("sonarqube", "quartz-quartz-bigbang-sonarqube", "", "username", "password", "sonarqube-sonarqube"),
			},
			"jenkins": {
				Description: "Jenkins",
				CallbackUrls: []ApplicationCallbackConfig{
					{Path: "/securityRealm/finishLogin"},
				},
				Lookup: NewApplicationLookupConfig("jenkins", "jenkins-admin-credentials", "", "username", "password", "jenkins"),
				Keycloak: map[string]interface{}{
					"mappers": map[string]interface{}{
						"username": map[string]interface{}{
							"protocol":       "openid-connect",
							"protocolMapper": "oidc-usermodel-property-mapper",
							"config": map[string]string{
								"user.attribute":            "username",
								"claim.name":                "preferred_username",
								"jsonType.label":            "String",
								"id.token.claim":            "true",
								"access.token.claim":        "true",
								"userinfo.token.claim":      "true",
								"introspection.token.claim": "true",
							},
						},
						"groups": map[string]interface{}{
							"protocol":       "openid-connect",
							"protocolMapper": "oidc-group-membership-mapper",
							"config": map[string]string{
								"claim.name":                "groups",
								"full.path":                 "true",
								"multivalued":               "true",
								"id.token.claim":            "true",
								"access.token.claim":        "true",
								"userinfo.token.claim":      "true",
								"introspection.token.claim": "true",
							},
						},
						"fullname": map[string]interface{}{
							"protocol":       "openid-connect",
							"protocolMapper": "oidc-full-name-mapper",
							"config": map[string]string{
								"id.token.claim":            "true",
								"access.token.claim":        "true",
								"userinfo.token.claim":      "true",
								"introspection.token.claim": "true",
							},
						},
					},
				},
			},
			"keycloak": {
				Description: "Keycloak",
				Db: InfrastructureApplicationDbConfig{
					Enabled:  true,
					Admin:    false,
					Username: "keycloak",
					DbName:   "keycloak",
				},
				Lookup: NewApplicationLookupConfig("keycloak", "keycloak-env", "", "KEYCLOAK_ADMIN", "KEYCLOAK_ADMIN_PASSWORD", "keycloak"),
			},
			"neuvector": {
				Description: "NeuVector",
				CallbackUrls: []ApplicationCallbackConfig{
					{Path: "/openId_auth"},
				},
				Lookup: NewApplicationLookupConfig("neuvector", "", "", "", "", "neuvector-neuvector"),
				Keycloak: map[string]interface{}{
					"mappers": map[string]interface{}{
						"realm roles": map[string]interface{}{
							"protocol":       "openid-connect",
							"protocolMapper": "oidc-usermodel-realm-role-mapper",
							"config": map[string]string{
								"user.attribute":            "foo",
								"claim.name":                "roles",
								"jsonType.label":            "String",
								"id.token.claim":            "true",
								"access.token.claim":        "true",
								"userinfo.token.claim":      "true",
								"introspection.token.claim": "true",
								"multivalued":               "true",
								"lightweight.claim":         "false",
							},
						},
					},
				},
			},
			"grafana": {
				Description: "Grafana",
				CallbackUrls: []ApplicationCallbackConfig{
					{Path: "/login/generic_oauth"},
				},
				Lookup: NewApplicationLookupConfig("monitoring", "monitoring-grafana", "", "admin-user", "admin-password", "monitoring-grafana-grafana"),
			},
			"tempo": {
				Description: "Tempo",
				CallbackUrls: []ApplicationCallbackConfig{
					{Path: "/login"},
				},
			},
		},
	}
}

func NewApplicationLookupConfig(ns string, adminSecret string, adminUsername string, adminUsernameKey string, adminPasswordKey string, ingressName string) ApplicationLookupConfig {
	c := ApplicationLookupConfig{
		Enabled: true,
		AdminCredentials: ApplicationLookupCredentialsConfig{
			Username: adminUsername,
			Secret: ApplicationLookupCredentialsSecretConfig{
				Name:        adminSecret,
				Namespace:   ns,
				UsernameKey: adminUsernameKey,
				PasswordKey: adminPasswordKey,
			},
		},
		Ingress: ApplicationLookupIngressConfig{
			Kind:      "VirtualService",
			Name:      ingressName,
			Namespace: ns,
		},
	}

	if c.AdminCredentials.Secret.UsernameKey == "" {
		c.AdminCredentials.Secret.UsernameKey = "username"
	}

	if c.AdminCredentials.Secret.PasswordKey == "" {
		c.AdminCredentials.Secret.PasswordKey = "password"
	}

	if c.Ingress.Name == "" {
		c.Ingress.Name = ns
	}

	return c
}

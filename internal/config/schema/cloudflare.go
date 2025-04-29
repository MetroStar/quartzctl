package schema

// CloudflareCredentials represents the credentials for accessing Cloudflare.
type CloudflareCredentials struct {
	AccountId string `koanf:"account_id"` // The Cloudflare account ID.
	ApiToken  string `koanf:"api_token"`  // The API token for Cloudflare.
	Email     string `koanf:"email"`      // The email associated with the Cloudflare account.
}

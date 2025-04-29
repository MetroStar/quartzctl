package schema

// IronbankCredentials represents the credentials for accessing Ironbank.
type IronbankCredentials struct {
	Username string `koanf:"username"`
	Password string `koanf:"password"`
	Email    string `koanf:"email"`
}

package schema

// DnsConfig represents the configuration for DNS in Quartz.
type DnsConfig struct {
	Zone   string `koanf:"zone"`   // The DNS zone.
	Domain string `koanf:"domain"` // The DNS domain.
}

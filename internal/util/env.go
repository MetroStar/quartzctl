package util

import (
	"os"
	"strings"
)

// GetenvOrDefault retrieves the value of the environment variable specified by `key`.
// If the variable is not set, it returns the provided `defaultValue`.
func GetenvOrDefault(key string, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return defaultValue
}

// OsEnvMap converts the current environment variables into a map.
func OsEnvMap() map[string]string {
	return EnvMap(os.Environ())
}

// EnvMap converts a slice of "key=value" strings into a map.
func EnvMap(environ []string) map[string]string {
	env := map[string]string{}
	for _, ev := range environ {
		parts := strings.SplitN(ev, "=", 2)
		if len(parts) == 0 {
			continue
		}
		k := parts[0]
		v := ""
		if len(parts) == 2 {
			v = parts[1]
		}
		env[k] = v
	}
	return env
}

package config

// Config holds the application configuration values.
type Config struct {
	Port string
}

// Load returns the application configuration.
func Load() Config {
	return Config{
		Port: "8082",
	}
}

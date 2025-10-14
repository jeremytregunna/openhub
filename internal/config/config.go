package config

type Config struct {
	StoragePath string
	SSHPort     int
	HTTPPort    int
	HTTPSPort   int
}

func Default() *Config {
	return &Config{
		StoragePath: "/var/lib/openhub/repos",
		SSHPort:     2222,
		HTTPPort:    3000,
		HTTPSPort:   3443,
	}
}

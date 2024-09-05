package config

type RawConfig struct {
	Port    int
	Proxies []map[string]interface{}
	Rules   []string
}


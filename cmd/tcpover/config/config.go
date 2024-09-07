package config

type RawConfig struct {
	Listen  string
	Proxies []map[string]interface{}
	Rules   []string
}

package dht

// Config dht config
type Config struct {
	Listen   uint16 // Default: 6881
	MaxNodes int    // Default: 10000
	MaxTX    int    // Default: 30000
	MaxToken int    // Default: 10000
}

// NewConfig create default config
func NewConfig() *Config {
	var cfg Config
	cfg.checkDefault()
	return &cfg
}

func (cfg *Config) checkDefault() {
	if cfg.Listen == 0 {
		cfg.Listen = 6881
	}
	if cfg.MaxNodes <= 0 {
		cfg.MaxNodes = 10000
	}
	if cfg.MaxTX <= 0 {
		cfg.MaxTX = 30000
	}
	if cfg.MaxToken <= 0 {
		cfg.MaxToken = 10000
	}
}

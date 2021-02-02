package dht

import "time"

// Config dht config
type Config struct {
	Listen            uint16        // Default: 6881
	MinNodes          int           // Default: 10000
	TxTimeout         time.Duration // Default: 30s
	MaxToken          int           // Default: 10000
	ShowTableInterval time.Duration // 0=not show, >0=interval Default: 0
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
	if cfg.MinNodes <= 0 {
		cfg.MinNodes = 10000
	}
	if cfg.TxTimeout <= 0 {
		cfg.TxTimeout = 30 * time.Second
	}
	if cfg.MaxToken <= 0 {
		cfg.MaxToken = 10000
	}
}

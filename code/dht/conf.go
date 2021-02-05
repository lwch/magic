package dht

import (
	"net"
	"time"
)

// Config dht config
type Config struct {
	Listen     uint16                      // Default: 6881
	MinNodes   int                         // Default: 10000
	MaxNodes   int                         // Default: 1000000
	TxTimeout  time.Duration               // Default: 30s
	GenID      func() [20]byte             // generate find id
	NodeFilter func(net.IP, [20]byte) bool // filter func for node id
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
	if cfg.MaxNodes <= 0 {
		cfg.MaxNodes = 1000000
	}
	if cfg.TxTimeout <= 0 {
		cfg.TxTimeout = 30 * time.Second
	}
}

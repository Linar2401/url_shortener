package config

import (
	"flag"
	"github.com/caarlos0/env/v6"
	"strings"
)

type Config struct {
	ServeAddress  string `env:"SERVER_ADDRESS"`
	ResultAddress string `env:"BASE_URL"`
}

func NewDefaultConfig() *Config {
	return &Config{
		ServeAddress:  "localhost:8080",
		ResultAddress: "http://localhost:8080",
	}
}

func Load() (*Config, error) {
	var cfg = NewDefaultConfig()
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	flag.StringVar(&cfg.ServeAddress, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(&cfg.ResultAddress, "b", "http://localhost:8080", "address and port to answer")
	flag.Parse()

	cfg.ServeAddress = strings.TrimSuffix(cfg.ServeAddress, "/")
	cfg.ResultAddress = strings.TrimSuffix(cfg.ResultAddress, "/")
	return cfg, nil
}

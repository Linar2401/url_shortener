package config

import (
	"flag"
	"strings"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	ServeAddress  string `env:"SERVER_ADDRESS"`
	ResultAddress string `env:"BASE_URL"`
	LogLevel      string `env:"LOG_LEVEL"`
}

func NewDefaultConfig() *Config {
	return &Config{
		ServeAddress:  "localhost:8080",
		ResultAddress: "http://localhost:8080",
		LogLevel:      "info",
	}
}

func Load() (*Config, error) {
	var cfg = NewDefaultConfig()
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	flag.StringVar(&cfg.ServeAddress, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(&cfg.ResultAddress, "b", "http://localhost:8080", "address and port to answer")
	flag.StringVar(&cfg.LogLevel, "log_level", "info", "Logging level")
	flag.Parse()

	cfg.ServeAddress = strings.TrimSuffix(cfg.ServeAddress, "/")
	cfg.ResultAddress = strings.TrimSuffix(cfg.ResultAddress, "/")

	return cfg, nil
}

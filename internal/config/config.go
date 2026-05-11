package config

import (
	"flag"
	"strings"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	ServeAddress    string `env:"SERVER_ADDRESS"`
	ResultAddress   string `env:"BASE_URL"`
	LogLevel        string `env:"LOG_LEVEL"`
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
}

func NewDefaultConfig() *Config {
	return &Config{
		ServeAddress:    "localhost:8080",
		ResultAddress:   "http://localhost:8080",
		LogLevel:        "info",
		FileStoragePath: "short-url-db.json",
	}
}

func Load() (*Config, error) {
	cfg := NewDefaultConfig()

	flag.StringVar(&cfg.ServeAddress, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(&cfg.ResultAddress, "b", "http://localhost:8080", "address and port to answer")
	flag.StringVar(&cfg.LogLevel, "log_level", "info", "Logging level")
	flag.StringVar(&cfg.FileStoragePath, "f", "short-url-db.json", "file storage path")
	flag.Parse()

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	cfg.ServeAddress = strings.TrimSuffix(cfg.ServeAddress, "/")
	cfg.ResultAddress = strings.TrimSuffix(cfg.ResultAddress, "/")

	return cfg, nil
}

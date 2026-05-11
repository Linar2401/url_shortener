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
	DatabaseDSN     string `env:"DATABASE_DSN"`
	AuthSecret      string `env:"AUTH_SECRET"`
	AuditFile       string `env:"AUDIT_FILE"`
	AuditURL        string `env:"AUDIT_URL"`
}

func NewDefaultConfig() *Config {
	return &Config{
		ServeAddress:    "localhost:8080",
		ResultAddress:   "http://localhost:8080",
		LogLevel:        "info",
		FileStoragePath: "",
		DatabaseDSN:     "",
		AuthSecret:      "url-shortener-default-secret",
	}
}

func Load() (*Config, error) {
	cfg := NewDefaultConfig()

	flag.StringVar(&cfg.ServeAddress, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(&cfg.ResultAddress, "b", "http://localhost:8080", "address and port to answer")
	flag.StringVar(&cfg.LogLevel, "log_level", "info", "Logging level")
	flag.StringVar(&cfg.FileStoragePath, "f", "", "file storage path")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "database DSN")
	flag.StringVar(&cfg.AuthSecret, "s", cfg.AuthSecret, "secret key for signing auth cookies")
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "path to audit log file (disabled if empty)")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "URL of remote audit sink (disabled if empty)")
	flag.Parse()

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	cfg.ServeAddress = strings.TrimSuffix(cfg.ServeAddress, "/")
	cfg.ResultAddress = strings.TrimSuffix(cfg.ResultAddress, "/")

	return cfg, nil
}

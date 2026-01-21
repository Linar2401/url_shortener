package config

import (
	"flag"
	"github.com/caarlos0/env/v6"
	"log"
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

func Load() *Config {
	var cfg Config
	err := env.Parse(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	flag.StringVar(&cfg.ServeAddress, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(&cfg.ResultAddress, "b", "http://localhost:8080", "address and port to answer")

	flag.Parse()

	if cfg.ServeAddress == "" {
		cfg.ServeAddress = strings.TrimRight(cfg.ServeAddress, "/")
	}

	if cfg.ResultAddress == "" {
		cfg.ResultAddress = strings.TrimRight(cfg.ResultAddress, "/")
	}

	return &cfg
}

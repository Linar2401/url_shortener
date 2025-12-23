package config

import (
	"flag"
)

type Config struct {
	ServeAddress  string
	ResultAddress string
}

func NewConfig() *Config {
	return &Config{
		ServeAddress:  "localhost:8080",
		ResultAddress: "localhost:8080",
	}
}

func Load() *Config {
	cfg := NewConfig()

	flag.StringVar(&cfg.ServeAddress, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(&cfg.ResultAddress, "b", "localhost:8080", "address and port to answer")

	flag.Parse()

	return cfg
}

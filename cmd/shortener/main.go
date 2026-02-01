package main

import (
	"log"

	"github.com/Linar2401/url_shortener/internal/config"
	"github.com/Linar2401/url_shortener/internal/handler"
	"github.com/Linar2401/url_shortener/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	if err := logger.Initialize("info"); err != nil {
		log.Fatal(err)
	}

	if err := handler.Serve(cfg); err != nil {
		log.Fatal(err)
	}
}

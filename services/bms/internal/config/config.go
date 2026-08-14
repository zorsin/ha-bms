package config

import (
	"log/slog"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/zorsin/ha-bms/internal/logger"
)

type Config struct {
	HTTPPort  int    `env:"HTTP_PORT"`
	HAURL     string `env:"HOMEASSISTANT_URL"`
	HAToken   string `env:"HOMEASSISTANT_TOKEN"`
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	BMSActive bool   `env:"BMS_ACTIVE" envDefault:"true"`
}

func Get() *Config {
	log := logger.Get()
	err := godotenv.Load()
	if err != nil {
		log.Debug("No .env found", slog.Any("err", err))
		//
	}

	// parse
	var cfg Config
	err = env.Parse(&cfg)
	if err != nil {
		log.Warn("Failed parsing config", slog.Any("err", err))
		return nil
	}

	return &cfg
}

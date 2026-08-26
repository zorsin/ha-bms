package config

import (
	"log/slog"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/zorsin/ha-bms/internal/logger"
)

type Config struct {
	HTTPPort        int    `env:"HTTP_PORT"`
	HAURL           string `env:"HOMEASSISTANT_URL"`
	HAMQTTURL       string `env:"HOMEASSISTANT_MQTT_URL"`
	HAMQTTUser      string `env:"HOMEASSISTANT_MQTT_USER"`
	HAMQTTPassword  string `env:"HOMEASSISTANT_MQTT_PASSWORD"`
	HAToken         string `env:"HOMEASSISTANT_TOKEN"`
	LogLevel        string `env:"LOG_LEVEL" envDefault:"info"`
	BMSActive       bool   `env:"BMS_ACTIVE" envDefault:"true"`
	MQTTSetDefaults bool   `env:"MQTT_SET_DEFAULTS" envDefault:"false"`
	EnergyChartsURL string `env:"ENERGY_CHARTS_URL" envDefault:"https://api.energy-charts.info/v2"`
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

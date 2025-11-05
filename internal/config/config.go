package config

import (
	"flag"

	"github.com/caarlos0/env"
)

const (
	IDMaxLength          int   = 8
	OriginalURLMaxLength int64 = 2048
)

type Config struct {
	ServerAddr       string `env:"SERVER_ADDRESS"`
	ShortURLBaseAddr string `env:"BASE_URL"`
	LogLevel         string `env:"LOG_LEVEL"`
	FileStoragePath  string `env:"FILE_STORAGE_PATH"`
}

func New() (*Config, error) {
	c := &Config{}

	flag.StringVar(&c.ServerAddr, "a", "localhost:8080", "server host:port")
	flag.StringVar(&c.ShortURLBaseAddr, "b", "http://localhost:8080", "base short url")
	flag.StringVar(&c.LogLevel, "l", "info", "log level")
	flag.StringVar(&c.FileStoragePath, "f", "shortener_storage.json", "file storage path")
	flag.Parse()

	if err := env.Parse(c); err != nil {
		return nil, err
	}

	return c, nil
}

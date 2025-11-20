package config

import (
	"flag"

	"github.com/caarlos0/env"
	"github.com/dsnikitin/shortener/internal/config/db"
)

const (
	IDMaxLength                int   = 8
	OriginalURLMaxLength       int64 = 2048
	MaxOriginalURLCountInBatch int64 = 500
)

type Config struct {
	ServerAddr       string `env:"SERVER_ADDRESS"`
	ShortURLBaseAddr string `env:"BASE_URL"`
	LogLevel         string `env:"LOG_LEVEL"`
	FileStoragePath  string `env:"FILE_STORAGE_PATH"`
	DataBase         *db.Config
}

func New() (*Config, error) {
	c := &Config{
		DataBase: &db.Config{},
	}

	flag.StringVar(&c.ServerAddr, "a", "localhost:8080", "server host:port")
	flag.StringVar(&c.ShortURLBaseAddr, "b", "http://localhost:8080", "base short url")
	flag.StringVar(&c.LogLevel, "l", "info", "log level")
	flag.StringVar(&c.FileStoragePath, "f", "shortener_storage.json", "file storage path")
	flag.StringVar(&c.DataBase.DSN, "d", "", "database dsn string")
	flag.Parse()

	if err := env.Parse(c); err != nil {
		return nil, err
	}

	if err := env.Parse(c.DataBase); err != nil {
		return nil, err
	}

	return c, nil
}

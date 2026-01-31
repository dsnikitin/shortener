package config

import (
	"flag"

	"github.com/caarlos0/env"
	"github.com/dsnikitin/shortener/internal/config/audit"
	"github.com/dsnikitin/shortener/internal/config/db"
)

const (
	IDMaxLength               int   = 8
	OriginalURLMaxLength      int64 = 2048
	OriginalURLMaxBatchLength int64 = 2048000
)

type Config struct {
	ServerAddr       string        `env:"SERVER_ADDRESS"`
	ShortURLBaseAddr string        `env:"BASE_URL"`
	LogLevel         string        `env:"LOG_LEVEL"`
	FileStoragePath  string        `env:"FILE_STORAGE_PATH"`
	JWTSigningKey    string        `env:"JWT_SIGNING_KEY"`
	DataBase         *db.Config    `envPrefix:"DATABASE_"`
	Audit            *audit.Config `envPrefix:"AUDIT_"`
}

func New() (*Config, error) {
	cfg := &Config{
		DataBase: &db.Config{},
		Audit:    &audit.Config{},
	}

	flag.StringVar(&cfg.ServerAddr, "a", "localhost:8080", "server host:port")
	flag.StringVar(&cfg.ShortURLBaseAddr, "b", "http://localhost:8080", "base short url")
	flag.StringVar(&cfg.LogLevel, "l", "info", "log level")
	flag.StringVar(&cfg.FileStoragePath, "f", "shortener_storage.json", "file storage path")
	flag.StringVar(&cfg.DataBase.DSN, "d", "", "database dsn string")
	flag.StringVar(&cfg.Audit.FilePath, "audit-file", "audit.json", "path to file where audit logs are saved")
	flag.StringVar(&cfg.Audit.URL, "audit-url", "", "full URL of remote server where audit logs are sent")
	flag.IntVar(&cfg.Audit.EventsLimit, "audit-events-limit", 1000, "audit events queue limit")
	flag.StringVar(&cfg.Audit.FileConsumerID, "audit-consumer-file", "file_consumer", "file consumer id of audit events")
	flag.StringVar(&cfg.Audit.RemoteConsumerID, "audit-consumer-remote", "remote_consumer", "remote consumer id of audit events")

	flag.Parse()

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

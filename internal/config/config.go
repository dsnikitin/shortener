package config

import (
	"flag"
	"strings"

	"github.com/caarlos0/env"

	"github.com/dsnikitin/shortener/internal/config/audit"
	"github.com/dsnikitin/shortener/internal/config/db"
)

const (
	// IDMaxLength максимальная длина короткой ссылки.
	IDMaxLength int = 8
	// OriginalURLMaxLength максимальная длина оригинального URL.
	OriginalURLMaxLength int64 = 2048
	// OriginalURLMaxBatchLength максимальная общая длина оригинальных URL в батче.
	OriginalURLMaxBatchLength int64 = 2048000
)

// Config содержит конфигурацию приложения.
type Config struct {
	ServerAddr         string        `env:"SERVER_ADDRESS"`
	ShortURLBaseAddr   string        `env:"BASE_URL"`
	LogLevel           string        `env:"LOG_LEVEL"`
	FileStoragePath    string        `env:"FILE_STORAGE_PATH"`
	JWTSigningKey      string        `env:"JWT_SIGNING_KEY"`
	IsDevelop          bool          `env:"IS_DEVELOP"`
	DataBase           *db.Config    `envPrefix:"DATABASE_"`
	Audit              *audit.Config `envPrefix:"AUDIT_"`
	EnableHTTPS        bool          `env:"ENABLE_HTTPS"`
	CertFilePath       string        `env:"CERT_FILE_PATH"`
	PrivateKeyFilePath string        `env:"PRIVATE_KEY_FILE_PATH"`
}

// New создает и инициализирует конфигурацию приложения.
func New() (*Config, error) {
	cfg := &Config{
		DataBase: &db.Config{},
		Audit:    &audit.Config{},
	}

	flag.StringVar(&cfg.ServerAddr, "a", "localhost:8080", "server host:port")
	flag.StringVar(&cfg.ShortURLBaseAddr, "b", "http://localhost:8080", "base short url")
	flag.StringVar(&cfg.LogLevel, "l", "info", "log level")
	flag.StringVar(&cfg.FileStoragePath, "f", "shortener_storage.json", "file storage path")
	flag.BoolVar(&cfg.IsDevelop, "dev", false, "is develop environment")
	flag.StringVar(&cfg.DataBase.DSN, "d", "", "database dsn string")
	flag.StringVar(&cfg.Audit.FilePath, "audit-file", "audit.json", "path to file where audit logs are saved")
	flag.StringVar(&cfg.Audit.URL, "audit-url", "", "full URL of remote server where audit logs are sent")
	flag.IntVar(&cfg.Audit.EventsLimit, "audit-events-limit", 1000, "audit events queue limit")
	flag.StringVar(&cfg.Audit.FileConsumerID, "audit-consumer-file", "file_consumer", "file consumer id of audit events")
	flag.StringVar(&cfg.Audit.RemoteConsumerID, "audit-consumer-remote", "remote_consumer", "remote consumer id of audit events")
	flag.BoolVar(&cfg.EnableHTTPS, "s", false, "enable https")
	flag.StringVar(&cfg.CertFilePath, "cert-file", "", "path to file with cert for https")
	flag.StringVar(&cfg.PrivateKeyFilePath, "key-file", "", "path to file with private key for https")

	flag.Parse()

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	if cfg.EnableHTTPS {
		cfg.ServerAddr = strings.Replace(cfg.ServerAddr, ":8080", ":8443", 1)
		cfg.ShortURLBaseAddr = strings.Replace(cfg.ShortURLBaseAddr, "http://", "https://", 1)
		cfg.ShortURLBaseAddr = strings.Replace(cfg.ShortURLBaseAddr, ":8080", ":8443", 1)

		if cfg.IsDevelop {
			if cfg.CertFilePath == "" {
				cfg.CertFilePath = "./certs/cert.pem"
			}
			if cfg.PrivateKeyFilePath == "" {
				cfg.PrivateKeyFilePath = "./certs/key.pem"
			}
		}
	}

	return cfg, nil
}

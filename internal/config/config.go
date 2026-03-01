package config

import (
	"encoding/json"
	"flag"
	"net/url"
	"os"

	"dario.cat/mergo"
	"github.com/caarlos0/env"
	"github.com/pkg/errors"

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
	ServerAddr         string        `env:"SERVER_ADDRESS" json:"server_address"`
	ShortURLBaseAddr   string        `env:"BASE_URL" json:"base_url"`
	LogLevel           string        `env:"LOG_LEVEL" json:"log_level"`
	FileStoragePath    string        `env:"FILE_STORAGE_PATH" json:"file_storage_path"`
	JWTSigningKey      string        `env:"JWT_SIGNING_KEY" json:"jwt_signing_key"`
	IsDevelop          bool          `env:"IS_DEVELOP" json:"is_develop"`
	EnableHTTPS        bool          `env:"ENABLE_HTTPS" json:"enable_https"`
	CertFilePath       string        `env:"CERT_FILE_PATH" json:"cert_file_path"`
	PrivateKeyFilePath string        `env:"PRIVATE_KEY_FILE_PATH" json:"private_key_file_path"`
	DataBase           *db.Config    `envPrefix:"DATABASE_" json:"database"`
	Audit              *audit.Config `envPrefix:"AUDIT_" json:"audit"`
	ConfigFilePath     string        `env:"CONFIG" json:"-"`
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
	flag.StringVar(&cfg.ConfigFilePath, "c", "", "path to config file")
	flag.StringVar(&cfg.ConfigFilePath, "config", "", "path to config file")

	flag.Parse()

	if cfg.ConfigFilePath != "" {
		if err := loadFromJSONFile(cfg); err != nil {
			return nil, errors.Wrap(err, "load from json file")
		}
	}

	if err := env.Parse(cfg); err != nil {
		return nil, errors.Wrap(err, "parse envs")
	}

	if cfg.EnableHTTPS {
		if baseURL, err := url.Parse(cfg.ShortURLBaseAddr); err != nil {
			return nil, errors.Wrap(err, "parse short url base address")
		} else if baseURL.Scheme != "https" {
			return nil, errors.New("Short url base address must use https scheme")
		}

		if cfg.CertFilePath == "" {
			return nil, errors.New("empty cert file path")
		}

		if cfg.PrivateKeyFilePath == "" {
			return nil, errors.New("empty private key file path")
		}
	}

	return cfg, nil
}

// loadFromJSONFile загружает конфигурацию из json-файла и записывает в пустые поля структуры Config.
func loadFromJSONFile(cfg *Config) error {
	data, err := os.ReadFile(cfg.ConfigFilePath)
	if err != nil {
		return errors.Wrap(err, "read config file")
	}

	fileCfg := Config{
		DataBase: &db.Config{},
		Audit:    &audit.Config{},
	}

	if err = json.Unmarshal(data, &fileCfg); err != nil {
		return errors.Wrap(err, "unmarshal config file data")
	}

	// заполняет только пустые поля в cfg значениями из fileCfg
	err = mergo.Merge(cfg, fileCfg)
	return errors.Wrap(err, "merge configs")
}

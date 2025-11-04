package config

import (
	"flag"
	"os"
)

const (
	IDMaxLength          int   = 8
	OriginalURLMaxLength int64 = 2048
)

type Config struct {
	ServerAddr       string
	ShortURLBaseAddr string
	LogLevel         string
	FileStoragePath  string
}

func NewFromArgs() *Config {
	c := Config{}

	flag.StringVar(&c.ServerAddr, "a", "localhost:8080", "server host:port")
	flag.StringVar(&c.ShortURLBaseAddr, "b", "http://localhost:8080", "base short url")
	flag.StringVar(&c.LogLevel, "l", "info", "log level")
	flag.StringVar(&c.FileStoragePath, "f", "shortener_storage.json", "file storage path")
	flag.Parse()

	if envServerAddr := os.Getenv("SERVER_ADDRESS"); envServerAddr != "" {
		c.ServerAddr = envServerAddr
	}
	if envBaseURL := os.Getenv("BASE_URL"); envBaseURL != "" {
		c.ShortURLBaseAddr = envBaseURL
	}
	if envLogLevel := os.Getenv("LOG_LEVEL"); envLogLevel != "" {
		c.LogLevel = envLogLevel
	}
	if envFileStoragePath := os.Getenv("FILE_STORAGE_PATH"); envFileStoragePath != "" {
		c.FileStoragePath = envFileStoragePath
	}

	return &c
}

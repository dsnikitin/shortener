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
}

func NewFromArgs() *Config {
	c := Config{}

	flag.StringVar(&c.ServerAddr, "a", "localhost:8080", "server host:port")
	flag.StringVar(&c.ShortURLBaseAddr, "b", "http://localhost:8080", "base short url")
	flag.Parse()

	if envServerAddr := os.Getenv("SERVER_ADDRESS"); envServerAddr != "" {
		c.ServerAddr = envServerAddr
	}
	if envBaseURL := os.Getenv("BASE_URL"); envBaseURL != "" {
		c.ShortURLBaseAddr = envBaseURL
	}

	return &c
}

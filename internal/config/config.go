package config

import (
	"flag"
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

	return &c
}

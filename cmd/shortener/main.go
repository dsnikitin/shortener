package main

import (
	"net/http"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/repository"
	"github.com/dsnikitin/shortener/internal/service"
)

func main() {
	c := config.NewFromArgs()
	r := repository.NewMemory()
	s := service.New(r)
	h := handler.New(c, s)

	if err := http.ListenAndServe(c.ServerAddr, newChiMux(h)); err != nil {
		panic(err)
	}
}

package main

import (
	"net/http"

	"github.com/dsnikitin/shortener/internal/config"
	"github.com/dsnikitin/shortener/internal/handler"
	"github.com/dsnikitin/shortener/internal/repository"
	"github.com/dsnikitin/shortener/internal/service"
)

func main() {
	r := repository.NewMemory()
	s := service.New(r)
	h := handler.New(s)

	if err := http.ListenAndServe(config.ServerAddr, newServeMux(h)); err != nil {
		panic(err)
	}
}

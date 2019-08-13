package api

import (
	"github.com/go-errors/errors"
	"github.com/gorilla/mux"
	"github.com/the-lightning-land/sweetd/dispenser"
	"net"
	"net/http"
)

type Config struct {
	Dispenser *dispenser.Dispenser
	Log       Logger
}

type Api struct {
	dispenser *dispenser.Dispenser
	router    *mux.Router
	log       Logger
}

func New(config *Config) *Api {
	api := &Api{
		router: mux.NewRouter(),
	}

	if config.Log != nil {
		api.log = config.Log
	} else {
		api.log = noopLogger{}
	}

	api.router.Handle("/api/v1/status", api.handleGetStatus()).Methods(http.MethodGet)
	api.router.Handle("/api/v1/update", api.handlePostUpdate()).Methods(http.MethodPost)

	// wifi conn

	return api
}

func (a *Api) SetDispenser(dispenser *dispenser.Dispenser) {
	a.dispenser = dispenser
}

func (a *Api) Serve(l net.Listener) error {
	err := http.Serve(l, a.router)
	if err != nil {
		return errors.Errorf("Unable to serve api: %v", err)
	}

	return nil
}

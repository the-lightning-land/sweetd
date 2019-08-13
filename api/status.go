package api

import (
	"net/http"
)

type getStatusResponse struct {
	This string `json:"this"`
}

func (a *Api) handleGetStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := &getStatusResponse{
			This: "that",
		}

		a.jsonResponse(w, res, http.StatusOK)
	}
}

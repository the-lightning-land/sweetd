package api

import (
	"encoding/json"
	"net/http"
)

type dispenserResponse struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	State   string `json:"state"`
}

type patchDispenserOp struct {
	Op string `json:"op"`
}

type patchDispenserRequest []patchDispenserOp

func (a *Handler) handleGetDispenser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := &dispenserResponse{
			Name: "that",
		}

		a.jsonResponse(w, res, http.StatusOK)
	}
}

func (a *Handler) handlePatchDispenser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := patchDispenserRequest{}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			a.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, op := range req {
			if op.Op == "reboot" {
				err := a.dispenser.Reboot()
				if err != nil {
					a.jsonError(w, "Could not reboot", http.StatusInternalServerError)
					return
				}
			} else if op.Op == "shutdown" {
				err := a.dispenser.ShutDown()
				if err != nil {
					a.jsonError(w, "Could not reboot", http.StatusInternalServerError)
					return
				}
			}
		}

		res := &dispenserResponse{
			Name: "that",
		}

		a.jsonResponse(w, res, http.StatusOK)
	}
}

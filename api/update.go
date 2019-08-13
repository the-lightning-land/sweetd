package api

import (
	"encoding/json"
	"net/http"
)

type postUpdateRequest struct {
	Url string `json:"url"`
}

type postUpdateResponse struct {
}

func (a *Api) handlePostUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := postUpdateRequest{}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			a.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = a.dispenser.Update(req.Url)
		if err != nil {
			a.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		a.jsonResponse(w, postUpdateResponse{}, http.StatusOK)
	}
}

func (a *Api) UpdateProgress() {
	a.dispenser.SubscribeDispenses()
}

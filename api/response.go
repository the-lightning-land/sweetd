package api

import (
	"encoding/json"
	"net/http"
)

func (a *Api) jsonResponse(w http.ResponseWriter, v interface{}, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		a.log.Errorf("Could not respond with JSON: %v", err)
	}
}

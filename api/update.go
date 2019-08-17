package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"net/http"
	"time"
)

type postUpdateRequest struct {
	Url string `json:"url"`
}

type postUpdateResponse struct {
	Id      string    `json:"id"`
	Started time.Time `json:"started"`
	Url     string    `json:"url"`
}

type getUpdateEventsEvent struct {
	Id           string    `json:"id"`
	Started      time.Time `json:"started"`
	Url          string    `json:"url"`
	State        string    `json:"state"`
	Progress     uint8     `json:"progress"`
	ShouldReboot bool      `json:"reboot"`
	ShouldCommit bool      `json:"commit"`
}

func (a *Api) handlePostUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := postUpdateRequest{}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			a.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		update, err := a.dispenser.Update(req.Url)
		if err != nil {
			a.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		a.jsonResponse(w, &postUpdateResponse{
			Id:      update.Id,
			Started: update.Started,
			Url:     update.Url,
		}, http.StatusOK)
	}
}

func (a *Api) handleGetUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := postUpdateRequest{}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			a.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		update, err := a.dispenser.Update(req.Url)
		if err != nil {
			a.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		a.jsonResponse(w, &postUpdateResponse{
			Id:      update.Id,
			Started: update.Started,
			Url:     update.Url,
		}, http.StatusOK)
	}
}

func (a *Api) handlePatchUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := postUpdateRequest{}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			a.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		update, err := a.dispenser.Update(req.Url)
		if err != nil {
			a.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		a.jsonResponse(w, &postUpdateResponse{
			Id:      update.Id,
			Started: update.Started,
			Url:     update.Url,
		}, http.StatusOK)
	}
}

func (a *Api) handleGetUpdateEvents() http.HandlerFunc {
	upgrader := &websocket.Upgrader{}

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		client, err := a.dispenser.SubscribeUpdate(id)
		if err != nil {
			a.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if client == nil {
			a.jsonError(w, fmt.Sprintf("No update with id %s found", id), http.StatusNotFound)
		}

		defer func() {
			err := client.Cancel()
			if err != nil {
				a.log.Errorf("Could not close client: %v", err)
			}
		}()

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			a.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// read pump
		go func() {
			defer c.Close()

			c.SetReadLimit(512)
			c.SetReadDeadline(time.Now().Add(60 * time.Second))
			c.SetPongHandler(func(string) error {
				c.SetReadDeadline(time.Now().Add(60 * time.Second))
				return nil
			})

			for {
				_, _, err := c.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						a.log.Errorf("unexpected websocket closure: %v", err)
					}
					break
				}
			}
		}()

		// write pump
		go func() {
			defer c.Close()

			ticker := time.NewTicker(54 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case update, ok := <-client.Update:
					c.SetWriteDeadline(time.Now().Add(10 * time.Second))

					if !ok {
						c.WriteMessage(websocket.CloseMessage, []byte{})
						return
					}

					err := c.WriteJSON(&getUpdateEventsEvent{
						Id:           update.Id,
						Started:      update.Started,
						Url:          update.Url,
						State:        update.State,
						Progress:     update.Progress,
						ShouldReboot: update.ShouldReboot,
						ShouldCommit: update.ShouldCommit,
					})
					if err != nil {
						return
					}
				case <-ticker.C:
					c.SetWriteDeadline(time.Now().Add(10 * time.Second))
					if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
						return
					}
				}
			}
		}()
	}
}

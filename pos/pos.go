package pos

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"github.com/go-errors/errors"
	"github.com/gobuffalo/packr/v2"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/the-lightning-land/sweetd/node"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Pos struct {
	log    Logger
	node   node.Node
	router *mux.Router
}

func NewPos(config *Config) (*Pos, error) {
	pos := &Pos{}

	if config.Logger != nil {
		pos.log = config.Logger
	} else {
		pos.log = noopLogger{}
	}

	pos.router = mux.NewRouter()
	pos.router.Use(pos.loggingMiddleware)

	api := pos.router.PathPrefix("/api").Subrouter()
	api.Use(pos.localhostMiddleware)
	api.Use(pos.availabilityMiddleware)
	api.Handle("/invoices/{rHash}/status", pos.handleStreamInvoiceStatus()).Methods(http.MethodGet)
	api.Handle("/invoices/{rHash}", pos.handleGetInvoice()).Methods(http.MethodGet)
	api.Handle("/invoices", pos.handleAddInvoice()).Methods(http.MethodPost)
	api.Use(mux.CORSMethodMiddleware(api))

	box := packr.New("web", "./out")
	pos.router.PathPrefix("/").Handler(pos.handleStatic(box)).Methods(http.MethodGet)

	return pos, nil
}

func (p *Pos) GenerateKey() (*rsa.PrivateKey, error) {
	// Generate a V2 RSA 1024 bit key
	return rsa.GenerateKey(rand.Reader, 1024)
}

func (p *Pos) Serve(l net.Listener) error {
	err := http.Serve(l, p.router)
	if err != nil {
		return errors.Errorf("Unable to serve PoS: %v", err)
	}

	return nil
}

func (p *Pos) SetNode(node node.Node) error {
	if p.node != nil {
		err := p.RemoveNode()
		if err != nil {
			p.log.Errorf("Could not remove previous node: %v", err)
		}
	}

	p.node = node

	return nil
}

func (p *Pos) RemoveNode() error {
	p.node = nil

	return nil
}

func (p *Pos) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.log.Infof("Accessing %v", r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func (p *Pos) localhostMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Referer(), "http://localhost:3001") {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3001")
			w.Header().Set("Access-Control-Max-Age", "1")
		}
		next.ServeHTTP(w, r)
	})
}

func (p *Pos) availabilityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p.node == nil {
			p.log.Errorf("PoS request failed due to unavailable node")
			p.jsonError(w, "No node is available at the moment", http.StatusServiceUnavailable)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (p *Pos) handleStatic(box *packr.Box) http.Handler {
	return http.FileServer(box)
}

func checkOrigin(r *http.Request) bool {
	origin := r.Header["Origin"]
	if len(origin) == 0 {
		return true
	}

	if strings.Contains(origin[0], "http://localhost:3001") {
		return true
	}

	u, err := url.Parse(origin[0])
	if err != nil {
		return false
	}

	return strings.EqualFold(u.Host, r.Host)
}

func (p *Pos) handleStreamInvoiceStatus() http.HandlerFunc {
	upgrader := &websocket.Upgrader{
		CheckOrigin: checkOrigin,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		rHash := vars["rHash"]

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			p.log.Errorf("Could not upgrade: %v", err)
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
						p.log.Errorf("Unexpected websocket closure: %v", err)
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

			client, err := p.node.SubscribeInvoices()
			if err != nil {
				p.log.Errorf("Could not subscribe to invoices: %v", err)
				return
			}

			defer client.Cancel()

			for {
				select {
				case invoice, ok := <-client.Invoices:
					c.SetWriteDeadline(time.Now().Add(10 * time.Second))

					if !ok {
						c.WriteMessage(websocket.CloseMessage, []byte{})
						return
					}

					if invoice.RHash != rHash {
						continue
					}

					err := c.WriteJSON(&invoiceStatusMessage{
						Settled: invoice.Settled,
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

func (p *Pos) handleGetInvoice() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		rHash := vars["rHash"]

		invoice, err := p.node.GetInvoice(rHash)
		if err != nil {
			p.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(&invoiceMessage{
			Settled:        invoice.Settled,
			RHash:          invoice.RHash,
			PaymentRequest: invoice.PaymentRequest,
		})
		if err != nil {
			p.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (p *Pos) handleAddInvoice() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		invoice, err := p.node.AddInvoice(&node.InvoiceRequest{
		})
		if err != nil {
			p.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(&invoiceMessage{
			Settled:        invoice.Settled,
			RHash:          invoice.RHash,
			PaymentRequest: invoice.PaymentRequest,
		})
		if err != nil {
			p.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

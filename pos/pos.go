package pos

import (
  "context"
  "encoding/json"
  "github.com/cretz/bine/control"
  "github.com/cretz/bine/tor"
  "github.com/go-errors/errors"
  "github.com/gobuffalo/packr/v2"
  "github.com/gorilla/mux"
  "github.com/gorilla/websocket"
  "github.com/the-lightning-land/sweetd/node"
  "log"
  "net"
  "net/http"
  "strings"
  "time"
)

type Pos struct {
  log      Logger
  listener net.Listener
  node     node.Node
  router   *mux.Router
  clients  []*client
  tor      *tor.Tor
  onion    *tor.OnionService
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
  api.Use(pos.availabilityMiddleware)
  api.PathPrefix("/").Handler(pos.handleApiCors()).Methods(http.MethodOptions)
  api.Handle("/invoices/status", pos.handleStreamInvoiceStatus()).Methods(http.MethodGet)
  api.Handle("/invoices/{rHash}", pos.handleGetInvoice()).Methods(http.MethodGet)
  api.Handle("/invoices", pos.handleAddInvoice()).Methods(http.MethodPost)
  api.Use(mux.CORSMethodMiddleware(api))

  box := packr.New("web", "./out")

  pos.router.Path("/").Handler(pos.handlePaymentsPage(box)).Methods(http.MethodGet)
  pos.router.PathPrefix("/").Handler(pos.handleStatic(box))

  return pos, nil
}

func (p *Pos) Start() error {
  var err error

  p.tor, err = tor.Start(nil, nil)
  if err != nil {
    return errors.Errorf("Could not start tor: %v", err)
  }

  key := control.GenKey(control.KeyAlgoED25519V3)

  lis, err := net.Listen("tcp", ":3000")
  if err != nil {
    return errors.New("Could not create listener for :3000")
  }

  p.listener = lis

  go func() {
    err = http.Serve(p.listener, p.router)
    if err != nil {
      p.log.Errorf("Server unable to listen on :3000")
    }
  }()

  go func() {
    listenCtx, listenCancel := context.WithTimeout(context.Background(), 3*time.Minute)
    defer listenCancel()

    p.onion, err = p.tor.Listen(listenCtx, &tor.ListenConf{
      // LocalPort:   3000,
      // RemotePorts: []int{3000},
      LocalListener: lis,
      Version3:      true,
      Key:           key,
      RemotePorts:   []int{80},
    })
    if err != nil {
      p.log.Errorf("Could not create onion service: %v", err)
    }

    p.log.Infof("Try http://%v.onion", p.onion.ID)

    p.log.Infof("Starting onion service...")

    err = http.Serve(p.onion, p.router)
    if err != nil {
      p.log.Errorf("Could not serve through onion service: %v", err)
    }

    p.log.Infof("Started onion service")
  }()

  return nil
}

func (p *Pos) Stop() error {
  err := p.listener.Close()
  if err != nil {
    return errors.New("Could not properly close listener")
  }

  err = p.onion.Close()
  if err != nil {
    return errors.Errorf("Could not properly close onion service: %v", err)
  }

  err = p.tor.Close()
  if err != nil {
    return errors.Errorf("Could not properly stop tor: %v", err)
  }

  return nil
}

func (p *Pos) SetNode(node node.Node) error {
  p.node = node

  // TODO: fix subscriptions

  return nil
}

func (p *Pos) loggingMiddleware(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    p.log.Infof("Accessing %v", r.RequestURI)
    next.ServeHTTP(w, r)
  })
}

func (p *Pos) availabilityMiddleware(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    p.log.Errorf("PoS request failed due to unavailable node")
    http.Error(w, "{ \"error\": \"No node is available at the moment\" }", http.StatusServiceUnavailable)
  })
}

func (p *Pos) handlePaymentsPage(box *packr.Box) http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {
    page := box.String("/pay/index.html")
    page = strings.ReplaceAll(page, "http://localhost:3000/api", r.URL.Hostname()+"/api")
    _, err := w.Write([]byte(page))
    if err != nil {
      http.Error(w, err.Error(), http.StatusInternalServerError)
    }
  }
}

func (p *Pos) handleStatic(box *packr.Box) http.Handler {
  return http.FileServer(box)
}

func (p *Pos) handleApiCors() http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3001")
    w.Header().Set("Access-Control-Max-Age", "1")
  }
}

func (p *Pos) handleStreamInvoiceStatus() http.HandlerFunc {
  upgrader := &websocket.Upgrader{}

  return func(w http.ResponseWriter, r *http.Request) {
    c, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
      log.Print("upgrade:", err)
      return
    }

    defer c.Close()

    client := &client{conn: c}
    p.clients = append(p.clients, client)

    // client.process()
  }
}

func (p *Pos) handleGetInvoice() http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    rHash := vars["rHash"]

    invoice, err := p.node.GetInvoice(rHash)
    if err != nil {
      http.Error(w, err.Error(), http.StatusInternalServerError)
      return
    }

    payload, err := json.Marshal(invoice)
    if err != nil {
      http.Error(w, err.Error(), http.StatusInternalServerError)
      return
    }

    _, err = w.Write(payload)
    if err != nil {
      http.Error(w, err.Error(), http.StatusInternalServerError)
      return
    }
  }
}

func (p *Pos) handleAddInvoice() http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {
    invoice, err := p.node.AddInvoice()
    if err != nil {
      http.Error(w, err.Error(), http.StatusInternalServerError)
      return
    }

    payload, err := json.Marshal(invoice)
    if err != nil {
      http.Error(w, err.Error(), http.StatusInternalServerError)
      return
    }

    _, err = w.Write(payload)
    if err != nil {
      http.Error(w, err.Error(), http.StatusInternalServerError)
      return
    }
  }
}

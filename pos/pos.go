package pos

import (
  "github.com/go-errors/errors"
  "github.com/gobuffalo/packr/v2"
  "net"
  "net/http"
)

type Pos struct {
  log      Logger
  listener net.Listener
  box      *packr.Box
}

func NewPos(config *Config) (*Pos, error) {
  pos := &Pos{}

  if config.Logger != nil {
    pos.log = config.Logger
  } else {
    pos.log = noopLogger{}
  }

  pos.box = packr.New("web", "./out")

  lis, err := net.Listen("tcp", ":3000")
  if err != nil {
    return nil, errors.New("Could not create listener for :3000")
  }

  pos.listener = lis

  return pos, nil
}

func (p *Pos) Start() error {
  go func() {
    err := http.Serve(p.listener, http.FileServer(p.box))
    if err != nil {
      p.log.Errorf("Server unable to listen on :3000")
    }
  }()

  return nil
}

func (p *Pos) Stop() error {
  err := p.listener.Close()
  if err != nil {
    return errors.New("Could not properly close listener")
  }

  return nil
}

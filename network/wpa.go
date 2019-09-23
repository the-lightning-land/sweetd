package network

import (
	"github.com/go-errors/errors"
	"github.com/the-lightning-land/sweetd/network/wpa"
	"sync"
)

// check WpaNetworks compliance to its interface during compile time
var _ Network = (*WpaNetwork)(nil)

type Config struct {
	Interface string
	Logger    Logger
}

type nextClient struct {
	sync.Mutex
	id uint32
}

type WpaNetwork struct {
	log        Logger
	wpa        *wpa.Wpa
	ifname     string
	iface      *wpa.Interface
	clients    map[uint32]*Client
	nextClient nextClient
}

func NewWpaNetwork(config *Config) *WpaNetwork {
	net := &WpaNetwork{
		ifname:  config.Interface,
		wpa:     wpa.New(),
		clients: make(map[uint32]*Client),
	}

	if config.Logger != nil {
		net.log = config.Logger
	} else {
		net.log = noopLogger{}
	}

	return net
}

func (n *WpaNetwork) Start() error {
	err := n.wpa.Start()
	if err != nil {
		return errors.Errorf("could not start wpa: %v", err)
	}

	iface, err := n.wpa.GetInterface(n.ifname)
	if err != nil {
		_ = n.Stop()
		return errors.Errorf("could not find interface %v: %v", n.ifname, err)
	}

	n.iface = iface

	return nil
}

func (n *WpaNetwork) Stop() error {
	err := n.wpa.Stop()
	if err != nil {
		return errors.Errorf("could not stop wpa: %v", err)
	}

	return nil
}

func (n *WpaNetwork) Status() *Status {
	return &Status{
	}
}

type WpaPskConnection struct {
	Ssid string
	Psk  string
}

type WpaConnection struct {
	Ssid string
}

func (n *WpaNetwork) Connect(connection Connection) error {
	switch connection.(type) {
	case *WpaPskConnection:
		// n.wpa.
	case *WpaConnection:
		// .s
	}

	return nil
}

func (n *WpaNetwork) Scan() error {
	return nil
}

func (n *WpaNetwork) Subscribe() *Client {
	client := &Client{
		Updates:    make(chan *Connectivity),
		cancelChan: make(chan struct{}),
		network:    n,
	}

	n.nextClient.Lock()
	client.Id = n.nextClient.id
	n.nextClient.id++
	n.nextClient.Unlock()

	n.clients[client.Id] = client

	return client
}

func (n *WpaNetwork) deleteClient(id uint32) {
	delete(n.clients, id)
}

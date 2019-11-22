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

func (n *WpaNetwork) Connect(connection Connection) error {
	switch connection.(type) {
	case *WpaPskConnection:
	case *WpaConnection:
	}

	return nil
}

func (n *WpaNetwork) Scan() (*ScanClient, error) {
	err := n.iface.Scan()
	if err != nil {
		return nil, errors.Errorf("unable to scan: %v", err)
	}

	wifisChan := make(chan *Wifi)

	client, err := n.iface.BSSAdded()
	if err != nil {
		return nil, errors.Errorf("unable to listen for added wifis: %v", err)
	}

	doneClient, err := n.iface.ScanDone()
	if err != nil {
		return nil, errors.Errorf("unable to listen to scan completion: %v", err)
	}

	bsss, err := n.iface.BSSs()
	if err != nil {
		return nil, errors.Errorf("unable to get BSSs: %v", err)
	}

	go func() {
		for _, bss := range bsss {
			b, err := bss.GetAll()
			if err != nil {
				continue
			}

			wifisChan <- &Wifi{
				Ssid: b.Ssid,
			}
		}

		for {
			select {
			case bss, ok := <-client.BSSAdded:
				if !ok {
					close(wifisChan)
					return
				}

				b, err := bss.GetAll()
				if err != nil {
					continue
				}

				wifisChan <- &Wifi{
					Ssid: b.Ssid,
				}
			case done, ok := <-doneClient.ScanDone:
				if !ok {
					close(wifisChan)
					return
				}

				if done {
					client.Cancel()
					doneClient.Cancel()
				}
			}
		}
	}()

	return &ScanClient{
		Wifis: wifisChan,
		Cancel: func() {
			client.Cancel()
			doneClient.Cancel()
		},
	}, nil
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

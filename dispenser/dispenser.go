package dispenser

import (
	"context"
	"crypto/rsa"
	"github.com/cretz/bine/tor"
	"github.com/go-errors/errors"
	"github.com/the-lightning-land/sweetd/ap"
	"github.com/the-lightning-land/sweetd/machine"
	"github.com/the-lightning-land/sweetd/node"
	"github.com/the-lightning-land/sweetd/pos"
	"github.com/the-lightning-land/sweetd/sweetdb"
	"github.com/the-lightning-land/sweetd/sweetlog"
	"github.com/the-lightning-land/sweetd/updater"
	"net"
	"sync"
	"time"
)

type Dispenser struct {
	machine              machine.Machine
	AccessPoint          ap.Ap
	db                   *sweetdb.DB
	DispenseOnTouch      bool
	BuzzOnDispense       bool
	done                 chan struct{}
	payments             chan *node.Invoice
	LightningNodeUri     string
	dispenses            chan bool
	dispenseClients      map[uint32]*DispenseClient
	dispenseClientMtx    sync.Mutex
	nextDispenseClientID uint32
	memoPrefix           string
	updater              updater.Updater
	node                 node.Node
	invoicesClient       *node.InvoicesClient
	pos                  *pos.Pos
	posOnion             *tor.OnionService
	sweetLog             *sweetlog.SweetLog
	log                  Logger
	tor                  *tor.Tor
	apiListeners         []net.Listener
	apiOnion             *tor.OnionService
	api                  Api
}

type DispenseClient struct {
	Dispenses  chan bool
	Id         uint32
	cancelChan chan struct{}
	dispenser  *Dispenser
}

type Config struct {
	Machine     machine.Machine
	AccessPoint ap.Ap
	DB          *sweetdb.DB
	MemoPrefix  string
	Updater     updater.Updater
	Pos         *pos.Pos
	SweetLog    *sweetlog.SweetLog
	Logger      Logger
	Tor         *tor.Tor
	Api         Api
}

func NewDispenser(config *Config) *Dispenser {
	dispenser := &Dispenser{
		machine:         config.Machine,
		AccessPoint:     config.AccessPoint,
		db:              config.DB,
		DispenseOnTouch: true,
		BuzzOnDispense:  false,
		done:            make(chan struct{}),
		payments:        make(chan *node.Invoice),
		dispenses:       make(chan bool),
		dispenseClients: make(map[uint32]*DispenseClient),
		memoPrefix:      config.MemoPrefix,
		updater:         config.Updater,
		pos:             config.Pos,
		sweetLog:        config.SweetLog,
		log:             config.Logger,
		tor:             config.Tor,
		api:             config.Api,
	}

	config.Api.SetDispenser(dispenser)

	return dispenser
}

func (d *Dispenser) Run() error {
	d.log.Infof("Starting machine...")

	wifiConnection, err := d.db.GetWifiConnection()
	if err != nil {
		d.log.Warnf("Could not retrieve saved wifi connection: %v", err)
	}

	if wifiConnection != nil {
		d.log.Infof("Will attempt connecting to Wifi %v.", wifiConnection.Ssid)

		err := d.AccessPoint.ConnectWifi(wifiConnection.Ssid, wifiConnection.Psk)
		if err != nil {
			d.log.Warnf("Whoops, couldn't connect to wifi: %v", err)
		}
	} else {
		d.log.Infof("No saved Wifi connection available. Not connecting.")
	}

	// Signal successful startup with two short buzzer noises
	d.machine.DiagnosticNoise()

	node, err := d.db.GetLightningNode()
	if err != nil {
		return err
	}

	// connect to remote lightning node
	if node != nil {
		err := d.ConnectLndNode(node.Uri, node.Cert, node.Macaroon)
		if err != nil {
			d.log.Errorf("Could not connect to remote lightning node: %v", err)
		}
	}

	err = d.StartPos()
	if err != nil {
		d.log.Errorf("Could not start PoS: %v", err)
	}

	lis, err := net.Listen("tcp", ":9000")
	if err != nil {
		return errors.New("RPC server unable to listen on 0.0.0.0:9000")
	}

	d.apiListeners = append(d.apiListeners, lis)

	go func() {
		err = d.api.Serve(lis)
		if err != nil {
			d.log.Errorf("Could not serve api: %v", err)
		}
	}()

	// Notify all subscribed dispense clients
	go func() {
		for {
			on := <-d.dispenses

			for _, client := range d.dispenseClients {
				client.Dispenses <- on
			}
		}
	}()

	touches, err := d.machine.SubscribeTouches()
	if err != nil {
		return errors.Errorf("Could not subscribe to touch events: %v", err)
	}

	defer func() {
		err := touches.Cancel()
		if err != nil {
			d.log.Errorf("Could not close touch event subscription: %v", err)
		}
	}()

	for {
		select {
		case on := <-touches.Touches:
			// react on direct touch events of the machine
			d.log.Infof("Touch event %v", on)

			if d.DispenseOnTouch && on {
				d.ToggleDispense(true)
			} else {
				d.ToggleDispense(false)
			}

		case <-d.payments:
			// react on incoming payments
			dispense := 1500 * time.Millisecond

			d.log.Debugf("Dispensing for a duration of %v", dispense)

			d.ToggleDispense(true)
			time.Sleep(dispense)
			d.ToggleDispense(false)

		case <-d.done:
			// finish loop when program is done
			return nil
		}
	}
}

func (d *Dispenser) ToggleDispense(on bool) {
	// Always make sure that buzzing stops
	if d.BuzzOnDispense || !on {
		d.machine.ToggleBuzzer(on)
	}

	d.machine.ToggleMotor(on)

	d.dispenses <- on
}

func (d *Dispenser) SaveLndNode(uri string, certBytes []byte, macaroonBytes []byte) error {
	err := d.db.SetLightningNode(&sweetdb.LightningNode{
		Uri:      uri,
		Cert:     certBytes,
		Macaroon: macaroonBytes,
	})

	if err != nil {
		return errors.Errorf("Couldn not save lnd node connection: %v", err)
	}

	return nil
}

func (d *Dispenser) DeleteLndNode() error {
	err := d.db.SetLightningNode(nil)

	if err != nil {
		return errors.Errorf("Couldn not delete lnd node connection: %v", err)
	}

	return nil
}

func (d *Dispenser) ConnectLndNode(uri string, certBytes []byte, macaroonBytes []byte) error {
	return nil
	if d.node != nil {
		err := d.DisconnectLndNode()
		if err != nil {
			d.log.Warnf("Could not properly disconnect previous node: %v", err)
		}
	}

	d.log.Infof("Connecting to remote lightning node %v", uri)

	var err error
	d.node, err = node.NewLndNode(&node.LndNodeConfig{
		Uri:           uri,
		Logger:        d.log,
		CertBytes:     certBytes,
		MacaroonBytes: macaroonBytes,
	})
	if err != nil {
		return errors.Errorf("Could not create node: %v", err)
	}

	err = d.node.Start()
	if err != nil {
		return errors.Errorf("Could not start node: %v", err)
	}

	// save currently connected node uri
	d.LightningNodeUri = uri

	d.invoicesClient, err = d.node.SubscribeInvoices()
	if err != nil {
		return errors.Errorf("Could not subscribe to invoices: %v", err)
	}

	go func() {
		for {
			invoice := <-d.invoicesClient.Invoices
			d.payments <- invoice
		}
	}()

	return nil
}

func (d *Dispenser) DisconnectLndNode() error {
	d.log.Infof("Disconnecting from remote lightning node")

	if d.node != nil {
		err := d.invoicesClient.Cancel()
		if err != nil {
			d.log.Warnf("Could not unsubscribe from invoices: %v", err)
		}

		err = d.node.Stop()
		if err != nil {
			return errors.Errorf("Could not stop node: %v", err)
		}
	}

	d.LightningNodeUri = ""
	d.node = nil
	d.invoicesClient = nil

	return nil
}

func (d *Dispenser) SetWifiConnection(connection *sweetdb.WifiConnection) error {
	d.log.Infof("Setting Wifi connection")

	err := d.db.SetWifiConnection(connection)
	if err != nil {
		return errors.Errorf("Failed setting Wifi connection: %v", err)
	}

	return nil
}

func (d *Dispenser) GetName() (string, error) {
	d.log.Infof("Getting name")

	name, err := d.db.GetName()
	if err != nil {
		return "", errors.Errorf("Failed getting name: %v", err)
	}

	return name, nil
}

func (d *Dispenser) SetName(name string) error {
	d.log.Infof("Setting name")

	err := d.db.SetName(name)
	if err != nil {
		return errors.Errorf("Failed setting name: %v", err)
	}

	return nil
}

func (d *Dispenser) SetDispenseOnTouch(dispenseOnTouch bool) error {
	d.log.Infof("Setting dispense on touch")

	d.DispenseOnTouch = dispenseOnTouch

	err := d.db.SetDispenseOnTouch(dispenseOnTouch)
	if err != nil {
		return errors.Errorf("Failed setting dispense on touch: %v", err)
	}

	return nil
}

func (d *Dispenser) SetBuzzOnDispense(buzzOnDispense bool) error {
	d.log.Infof("Setting buzz on dispense")

	d.BuzzOnDispense = buzzOnDispense

	err := d.db.SetBuzzOnDispense(buzzOnDispense)
	if err != nil {
		return errors.Errorf("Failed setting buzz on dispense: %v", err)
	}

	return nil
}

func (d *Dispenser) ConnectToWifi(ssid string, psk string) error {
	d.log.Infof("Connecting to wifi %v", ssid)

	err := d.AccessPoint.ConnectWifi(ssid, psk)
	if err != nil {
		d.log.Errorf("Could not get Wifi networks: %v", err)
		return errors.New("Could not get Wifi networks")
	}

	err = d.SetWifiConnection(&sweetdb.WifiConnection{
		Ssid: ssid,
		Psk:  psk,
	})
	if err != nil {
		d.log.Errorf("Could not save wifi connection: %v", err)
	}

	return nil
}

func (d *Dispenser) StartPos() error {
	var key *rsa.PrivateKey

	d.log.Infof("Starting PoS")

	key, err := d.db.GetPosPrivateKey()
	if err != nil {
		d.log.Warnf("Could not read PoS private key: %v", err)
	}

	if key == nil {
		key, err = d.pos.GenerateKey()
		if err != nil {
			return errors.Errorf("Could not generate PoS private key: %v", err)
		}

		d.log.Infof("Generated new PoS private key")

		err := d.db.SetPosPrivateKey(key)
		if err != nil {
			d.log.Errorf("Could not save generated PoS private key: %v", err)
		}
	}

	err = d.pos.SetNode(d.node)
	if err != nil {
		return errors.Errorf("Could not set PoS node: %v", err)
	}

	listenCtx, listenCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer listenCancel()

	d.posOnion, err = d.tor.Listen(listenCtx, &tor.ListenConf{
		Key:         key,
		RemotePorts: []int{80},
	})
	if err != nil {
		return errors.Errorf("Could not create onion service: %v", err)
	}

	d.log.Infof("Try http://%v.onion", d.posOnion.ID)

	go func() {
		d.log.Infof("Starting onion service...")

		err = d.pos.Serve(d.posOnion)
		if err != nil {
			d.log.Errorf("Could not serve through onion service: %v", err)
		}

		d.log.Infof("Started onion service")
	}()

	return nil
}

func (d *Dispenser) GetApiOnionID() string {
	if d.apiOnion == nil {
		return ""
	}

	return d.apiOnion.ID
}

func (d *Dispenser) StopPos() error {
	d.log.Infof("Stopping PoS")

	return nil
}

func (d *Dispenser) Shutdown() {
	for _, lis := range d.apiListeners {
		err := lis.Close()
		if err != nil {
			d.log.Errorf("Could not close listener: %v", err)
		}
	}

	if d.node != nil {
		err := d.node.Stop()
		if err != nil {
			d.log.Warnf("Could not properly shut down node: %v", err)
		}

		d.node = nil
	}

	err := d.StopPos()
	if err != nil {
		d.log.Errorf("Could not stop PoS: %v", err)
	}

	close(d.done)
}

func (d *Dispenser) SubscribeDispenses() *DispenseClient {
	client := &DispenseClient{
		Dispenses:  make(chan bool),
		cancelChan: make(chan struct{}),
		dispenser:  d,
	}

	d.dispenseClientMtx.Lock()
	client.Id = d.nextDispenseClientID
	d.nextDispenseClientID++
	d.dispenseClientMtx.Unlock()

	d.dispenseClients[client.Id] = client

	return client
}

func (c *DispenseClient) Cancel() {
	delete(c.dispenser.dispenseClients, c.Id)

	close(c.cancelChan)
}

package dispenser

import (
	"github.com/cretz/bine/tor"
	"github.com/go-errors/errors"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/the-lightning-land/sweetd/api"
	"github.com/the-lightning-land/sweetd/app"
	"github.com/the-lightning-land/sweetd/lightning"
	"github.com/the-lightning-land/sweetd/machine"
	"github.com/the-lightning-land/sweetd/network"
	"github.com/the-lightning-land/sweetd/nodeman"
	"github.com/the-lightning-land/sweetd/onion"
	"github.com/the-lightning-land/sweetd/pos"
	"github.com/the-lightning-land/sweetd/reboot"
	"github.com/the-lightning-land/sweetd/sweetdb"
	"github.com/the-lightning-land/sweetd/sweetlog"
	"github.com/the-lightning-land/sweetd/updater"
	"net/http"
	"sync"
	"time"
)

type State string

const (
	StateStarting State = "starting"
	StateStarted        = "running"
	StateStopping       = "stopping"
	StateStopped        = "stopped"
)

type DispenseState int

const (
	DispenseStateOn DispenseState = iota
	DispenseStateOff
)

type nextClient struct {
	sync.Mutex
	id uint32
}

type Config struct {
	Machine  machine.Machine
	DB       *sweetdb.DB
	Updater  updater.Updater
	SweetLog *sweetlog.SweetLog
	Logger   *logrus.Entry
	Tor      *tor.Tor
	Network  network.Network
	Nodeman  *nodeman.Nodeman
}

type Dispenser struct {
	// machine handles the touch sensor and physical dispensing and buzzing
	machine machine.Machine

	// network manages network connections
	network network.Network

	// db holds all persistent data
	db *sweetdb.DB

	// name is the personalized name of the dispenser
	name string

	// dispenseOnTouch indicates if device should dispense on touch
	dispenseOnTouch bool

	// buzzOnDispense indicates if the dispenser should buzz during dispensing
	buzzOnDispense bool

	// apiOnionService
	apiOnionService *onion.Service

	// posOnionService
	posOnionService *onion.Service

	// done can be closed when the dispenser should be shutdown
	done chan struct{}

	// dispenses signals whenever
	dispenses chan DispenseState

	// payments
	payments chan *lightning.Invoice

	// subscribers to dispense events
	dispenseClients map[uint32]*DispenseClient

	// next dispense event client information
	nextClient nextClient

	// updater handles system updates
	updater updater.Updater

	// nodeman node manager
	nodeman *nodeman.Nodeman

	// TODO(davidknezic): replace this with logInterceptor or similar
	sweetLog *sweetlog.SweetLog

	log *logrus.Entry

	// tor provides access to the Tor network through which the
	// api and the point of sale is exposed
	tor *tor.Tor

	// posHandler
	posHandler http.Handler

	// apiHandler
	apiHandler http.Handler

	// state keeps track of the lifecycle of the dispenser
	state State
}

func NewDispenser(config *Config) *Dispenser {
	dispenser := &Dispenser{
		nodeman:         config.Nodeman,
		machine:         config.Machine,
		network:         config.Network,
		db:              config.DB,
		payments:        make(chan *lightning.Invoice),
		dispenseClients: make(map[uint32]*DispenseClient),
		updater:         config.Updater,
		sweetLog:        config.SweetLog,
		log:             config.Logger,
		tor:             config.Tor,
		state:           StateStopped,
		posOnionService: onion.NewService(&onion.ServiceConfig{
			Tor:    config.Tor,
			Logger: config.Logger.WithField("system", "onion").WithField("for", "pos"),
		}),
		apiOnionService: onion.NewService(&onion.ServiceConfig{
			Tor:    config.Tor,
			Logger: config.Logger.WithField("system", "onion").WithField("for", "api"),
		}),
	}

	dispenser.posHandler = pos.NewHandler(&pos.Config{
		Logger:    config.Logger.WithField("system", "pos"),
		Dispenser: dispenser,
	})

	apiHandler := api.NewHandler(&api.Config{
		Log:       config.Logger.WithField("system", "api"),
		Dispenser: dispenser,
	})

	appHandler := app.NewHandler(&app.Config{
		Logger: config.Logger.WithField("system", "app"),
	})

	router := mux.NewRouter()
	router.PathPrefix("/api/v1").Handler(http.StripPrefix("/api/v1", apiHandler))
	router.PathPrefix("/").Handler(appHandler)

	dispenser.apiHandler = router

	return dispenser
}

// restoreConfigs re-applies saved dispenser configs from the database
func (d *Dispenser) restoreConfigs() {
	name, err := d.db.GetName()
	if err != nil {
		d.log.Errorf("could not get name: %v", err)
	}

	d.name = name

	dispenseOnTouch, err := d.db.GetDispenseOnTouch()
	if err != nil {
		d.log.Errorf("could not get dispense on touch: %v", err)
	}

	d.dispenseOnTouch = dispenseOnTouch

	buzzOnDispense, err := d.db.GetBuzzOnDispense()
	if err != nil {
		d.log.Errorf("could not get buzz on dispense: %v", err)
	}

	d.buzzOnDispense = buzzOnDispense

	posPrivateKey, err := d.db.GetPosPrivateKey()
	if err != nil {
		d.log.Warnf("Could not read PoS private key: %v", err)
	}

	if posPrivateKey == nil {
		posPrivateKey, err = onion.GeneratePrivateKey(onion.V2)
		if err != nil {
			d.log.Errorf("Could not generate PoS private key: %v", err)
		}

		d.posOnionService.SetPrivateKey(posPrivateKey)
		d.log.Infof("created new pos address: %s.onion", d.posOnionService.ID())

		err := d.db.SetPosPrivateKey(posPrivateKey)
		if err != nil {
			d.log.Errorf("Could not save generated PoS private key: %v", err)
		}
	} else {
		d.posOnionService.SetPrivateKey(posPrivateKey)
		d.log.Infof("using saved pos address: %s.onion", d.posOnionService.ID())
	}

	apiPrivateKey, err := d.db.GetApiPrivateKey()
	if err != nil {
		d.log.Warnf("could not read api private key: %v", err)
	}

	if apiPrivateKey == nil {
		apiPrivateKey, err = onion.GeneratePrivateKey(onion.V2)
		if err != nil {
			d.log.Errorf("could not generate api private key: %v", err)
		}

		d.apiOnionService.SetPrivateKey(apiPrivateKey)
		d.log.Infof("created new api address: %s.onion", d.apiOnionService.ID())

		err := d.db.SetApiPrivateKey(apiPrivateKey)
		if err != nil {
			d.log.Errorf("could not save generated api private key: %v", err)
		}
	} else {
		d.apiOnionService.SetPrivateKey(apiPrivateKey)
		d.log.Infof("using saved api address: %s.onion", d.apiOnionService.ID())
	}
}

// handleDispenses is run as a goroutine and handles dispenses
func (d *Dispenser) handleDispenses(wg sync.WaitGroup) {
	wg.Add(1)

	d.log.Infof("started handling dispenses")

	touchesClient := d.machine.SubscribeTouches()
	done := false

	for !done {
		select {
		case on := <-touchesClient.Touches:
			// react on direct touch events of the machine
			d.log.Infof("Touch event %v", on)

			if d.dispenseOnTouch && on {
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
			done = true
		}
	}

	touchesClient.Cancel()

	d.log.Infof("stopped handling dispenses")

	wg.Done()
}

// notifyDispenseSubscribers is run as a goroutine and notifies all dispense
// subscribers when the dispense state changes
func (d *Dispenser) notifyDispenseSubscribers(wg sync.WaitGroup) {
	wg.Add(1)

	done := false

	for !done {
		select {
		case on := <-d.dispenses:
			for _, client := range d.dispenseClients {
				client.Dispenses <- on
			}
		case <-d.done:
			// finish loop when program is done
			done = true
		}
	}

	// cancel all client subscriptions
	for _, client := range d.dispenseClients {
		client.Cancel()
	}

	wg.Done()
}

// maybeAttemptSavedWifiConnection is run as a goroutine and attempts a connection
// to the most recently persisted wifi connection, if no network connection is available yet
func (d *Dispenser) maybeAttemptSavedWifiConnection(wg sync.WaitGroup) {
	wg.Add(1)

	wifiConnection, err := d.db.GetWifiConnection()
	if err != nil {
		d.log.Warnf("could not get wifi connection: %v", err)
	}

	if wifiConnection != nil {
		err := d.network.Connect(&network.WpaPskConnection{
			Ssid: wifiConnection.Ssid,
			Psk:  wifiConnection.Psk,
		})
		if err != nil {
			d.log.Errorf("could not connect to saved wifi: %v", err)
		}
	} else {
		d.log.Debugf("no saved wifi connection was found")
	}

	wg.Done()
}

// RunAndWait initializes all states and runs the dispenser in a blocking way until it is stopped
func (d *Dispenser) RunAndWait() error {
	var err error

	d.state = StateStarting

	// track tasks so function can be returned from only when all tasks are stopped
	var wg sync.WaitGroup

	// initialize a new channel that tracks dispense states
	d.dispenses = make(chan DispenseState)

	// initialize a new done channel to be closed to stop the dispenser
	d.done = make(chan struct{})

	// restore configs from the database
	d.restoreConfigs()

	// start background routines
	go d.maybeAttemptSavedWifiConnection(wg)
	go d.notifyDispenseSubscribers(wg)
	go d.runLightningNodes(wg)
	go d.handleDispenses(wg)

	err = d.runPos(wg)
	if err != nil {
		err = errors.Errorf("unable to run point of sales: %v", err)
		d.Stop()
		goto Teardown
	}

	err = d.runApi(wg)
	if err != nil {
		err = errors.Errorf("unable to run api: %v", err)
		d.Stop()
		goto Teardown
	}

	d.state = StateStarted

	// signal successful startup with two short buzzer noises
	d.machine.DiagnosticNoise()

	d.log.Infof("dispenser started")

	// block until the done channel is closed
	<-d.done

Teardown:
	d.state = StateStopping

	// tear off dispenses channel
	close(d.dispenses)
	d.dispenses = nil

	// wait for all registered tasks to finish
	wg.Wait()

	d.state = StateStopped

	return err
}

func (d *Dispenser) ToggleDispense(on bool) {
	// Always make sure that buzzing stops
	if d.buzzOnDispense || !on {
		d.machine.ToggleBuzzer(on)
	}

	d.machine.ToggleMotor(on)

	if on {
		d.dispenses <- DispenseStateOn
	} else {
		d.dispenses <- DispenseStateOff
	}
}

func (d *Dispenser) SetWifiConnection(connection *sweetdb.WifiConnection) error {
	d.log.Infof("Setting Wifi connection")

	err := d.db.SetWifiConnection(connection)
	if err != nil {
		return errors.Errorf("Failed setting Wifi connection: %v", err)
	}

	return nil
}

func (d *Dispenser) GetState() State {
	return d.state
}

func (d *Dispenser) GetName() string {
	if d.name == "" {
		// TODO: Name the dispenser individually by default
		// name = fmt.Sprintf("Candy %v", id)

		return "Candy Dispenser"
	}

	return d.name
}

func (d *Dispenser) ShouldDispenseOnTouch() bool {
	return d.dispenseOnTouch
}

func (d *Dispenser) ShouldBuzzOnDispense() bool {
	return d.buzzOnDispense
}

func (d *Dispenser) SetName(name string) error {
	d.log.Infof("Setting name")

	d.name = name

	err := d.db.SetName(name)
	if err != nil {
		return errors.Errorf("Failed setting name: %v", err)
	}

	return nil
}

func (d *Dispenser) SetDispenseOnTouch(dispenseOnTouch bool) error {
	d.log.Infof("Setting dispense on touch")

	d.dispenseOnTouch = dispenseOnTouch

	err := d.db.SetDispenseOnTouch(dispenseOnTouch)
	if err != nil {
		return errors.Errorf("Failed setting dispense on touch: %v", err)
	}

	return nil
}

func (d *Dispenser) SetBuzzOnDispense(buzzOnDispense bool) error {
	d.log.Infof("Setting buzz on dispense")

	d.buzzOnDispense = buzzOnDispense

	err := d.db.SetBuzzOnDispense(buzzOnDispense)
	if err != nil {
		return errors.Errorf("Failed setting buzz on dispense: %v", err)
	}

	return nil
}

func (d *Dispenser) ConnectToWifi(ssid string, psk string) error {
	d.log.Infof("Connecting to wifi %v", ssid)

	err := d.network.Connect(&network.WpaPskConnection{
		Ssid: ssid,
		Psk:  psk,
	})
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

func (d *Dispenser) Reboot() error {
	d.state = StateStopping

	err := reboot.Reboot()
	if err != nil {
		return errors.Errorf("Could not reboot: %v", err)
	}

	return nil
}

func (d *Dispenser) ShutDown() error {
	d.state = StateStopping

	err := reboot.ShutDown()
	if err != nil {
		return errors.Errorf("Could not shut down: %v", err)
	}

	return nil
}

func (d *Dispenser) Stop() {
	// signal the dispenser run loop to stop
	close(d.done)
}

func (d *Dispenser) SubscribeDispenses() *DispenseClient {
	client := &DispenseClient{
		Dispenses:  make(chan DispenseState),
		cancelChan: make(chan struct{}),
		dispenser:  d,
	}

	d.nextClient.Lock()
	client.Id = d.nextClient.id
	d.nextClient.id++
	d.nextClient.Unlock()

	d.dispenseClients[client.Id] = client

	return client
}

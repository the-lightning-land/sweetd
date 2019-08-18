package main

import (
	"github.com/cretz/bine/tor"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/the-lightning-land/sweetd/ap"
	"github.com/the-lightning-land/sweetd/api"
	"github.com/the-lightning-land/sweetd/dispenser"
	"github.com/the-lightning-land/sweetd/machine"
	"github.com/the-lightning-land/sweetd/pairing"
	"github.com/the-lightning-land/sweetd/pos"
	"github.com/the-lightning-land/sweetd/sweetdb"
	"github.com/the-lightning-land/sweetd/sweetlog"
	"github.com/the-lightning-land/sweetd/updater"
	"net/http"
	"os"
	"os/signal"
	// Blank import to set up profiling HTTP handlers.
	_ "net/http/pprof"
)

var (
	// commit stores the current commit hash of this build. This should be set using -ldflags during compilation.
	Commit string
	// version stores the version string of this build. This should be set using -ldflags during compilation.
	Version string
	// date stores the date of this build. This should be set using -ldflags during compilation.
	Date string
)

// sweetdMain is the true entry point for sweetd. This is required since defers
// created in the top-level scope of a main method aren't executed if os.Exit() is called.
func sweetdMain() error {
	sweetLog := sweetlog.New()

	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
	log.AddHook(sweetLog)

	// Load CLI configuration and defaults
	cfg, err := loadConfig()
	if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
		return nil
	} else if err != nil {
		return errors.Errorf("Failed parsing arguments: %v", err)
	}

	// Set logger into debug mode if called with --debug
	if cfg.Debug {
		log.SetLevel(log.DebugLevel)
		log.Info("Setting debug mode.")
	}

	log.Debug("Loaded config.")

	// Print version of the daemon
	log.Infof("Version %s (commit %s)", Version, Commit)
	log.Infof("Built on %s", Date)

	// Stop here if only version was requested
	if cfg.ShowVersion {
		return nil
	}

	if cfg.Profiling != nil {
		go func() {
			log.Infof("Starting profiling server on %v", cfg.Profiling.Listen)
			// Redirect the root path
			http.Handle("/", http.RedirectHandler("/debug/pprof", http.StatusSeeOther))
			// All other handlers are registered on DefaultServeMux through the import of pprof
			err := http.ListenAndServe(cfg.Profiling.Listen, nil)
			if err != nil {
				log.Errorf("Could not run profiler: %v", err)
			}
		}()
	}

	// sweet.db persistently stores all dispenser configurations and settings
	sweetDB, err := sweetdb.Open(cfg.DataDir)
	if err != nil {
		return errors.Errorf("Could not open sweet.db: %v", err)
	}

	log.Infof("Opened sweet.db")

	defer func() {
		err := sweetDB.Close()
		if err != nil {
			log.Errorf("Could not close sweet.db: %v", err)
		} else {
			log.Info("Closed sweet.db.")
		}
	}()

	// The network access point, which acts as the core connectivity
	// provider for all other components
	var a ap.Ap

	switch cfg.Net {
	case "dispenser":
		a, err = ap.NewDispenserAp(&ap.DispenserApConfig{
			Interface: "wlan0",
		})

		log.Info("Created Candy Dispenser access point.")
	case "mock":
		a = ap.NewMockAp()

		log.Info("Created a mock access point.")
	default:
		return errors.Errorf("Unknown networking type %v", cfg.Machine)
	}

	err = a.Start()
	if err != nil {
		return errors.Errorf("Could not start access point: %v", err)
	}

	defer func() {
		err := a.Stop()
		if err != nil {
			log.Errorf("Could not properly shut down access point: %v", err)
		} else {
			log.Info("Stopped access point.")
		}
	}()

	// The updater
	var u updater.Updater

	switch cfg.Updater {
	case "none":
		u = updater.NewNoopUpdater()

		log.Info("Created noop updater.")
	case "mender":
		u, err = updater.NewMenderUpdater(&updater.MenderUpdaterConfig{
			Logger: log.New().WithField("system", "updater"),
			DB:     sweetDB,
		})

		log.Info("Created Mender updater.")
	default:
		return errors.Errorf("Unknown updater type %v", cfg.Updater)
	}

	// The hardware controller
	var m machine.Machine

	switch cfg.Machine {
	case "raspberry":
		m = machine.NewDispenserMachine(&machine.DispenserMachineConfig{
			TouchPin:  cfg.Raspberry.TouchPin,
			MotorPin:  cfg.Raspberry.MotorPin,
			BuzzerPin: cfg.Raspberry.BuzzerPin,
		})

		log.Infof("Created Raspberry Pi machine on touch pin %v, motor pin %v and buzzer pin %v.",
			cfg.Raspberry.TouchPin, cfg.Raspberry.MotorPin, cfg.Raspberry.BuzzerPin)
	case "mock":
		m = machine.NewMockMachine(cfg.Mock.Listen)

		log.Info("Created a mock machine.")
	default:
		return errors.Errorf("Unknown machine type %v", cfg.Machine)
	}

	if err := m.Start(); err != nil {
		return errors.Errorf("Could not start machine: %v", err)
	}

	defer func() {
		err := m.Stop()
		if err != nil {
			log.Errorf("Could not properly stop machine: %v", err)
		} else {
			log.Infof("Stopped machine.")
		}
	}()

	// start Tor node
	t, err := tor.Start(nil, &tor.StartConf{
		ExePath:         cfg.Tor.Path,
		TempDataDirBase: os.TempDir(),
		DebugWriter:     log.New().WithField("system", "tor").WriterLevel(log.DebugLevel),
	})
	if err != nil {
		return errors.Errorf("Could not start tor: %v", err)
	}

	log.Infof("Started Tor.")

	defer func() {
		err := t.Close()
		if err != nil {
			log.Errorf("Could not properly stop Tor: %v", err)
		} else {
			log.Infof("Stopped Tor.")
		}
	}()

	// create subsystem responsible for the point of sale app
	pos, err := pos.NewPos(&pos.Config{
		Logger: log.New().WithField("system", "pos"),
	})
	if err != nil {
		return errors.Errorf("Could not create PoS: %v", err)
	}

	log.Infof("Created PoS")

	// create subsystem responsible for the point of sale app
	api := api.New(&api.Config{
		Log: log.New().WithField("system", "api"),
	})
	if err != nil {
		return errors.Errorf("Could not create api: %v", err)
	}

	log.Infof("Created API")

	// central controller for everything the dispenser does
	dispenser := dispenser.NewDispenser(&dispenser.Config{
		Machine:     m,
		AccessPoint: a,
		DB:          sweetDB,
		MemoPrefix:  cfg.MemoPrefix,
		Updater:     u,
		Pos:         pos,
		SweetLog:    sweetLog,
		Logger:      log.New().WithField("system", "dispenser"),
		Tor:         t,
		Api:         api,
	})

	log.Infof("Created dispenser.")

	// create subsystem responsible for pairing
	pairingController, err := pairing.NewController(&pairing.Config{
		Logger:      log.New().WithField("system", "pairing"),
		AdapterId:   "hci0",
		AccessPoint: a,
		Dispenser:   dispenser,
	})
	if err != nil {
		return errors.Errorf("Could not create pairing controller: %v", err)
	}

	log.Infof("Created pairing controller.")

	err = pairingController.Start()
	if err != nil {
		return errors.Errorf("Could not start pairing controller: %v", err)
	}

	log.Infof("Started pairing controller.")

	defer func() {
		err := pairingController.Stop()
		if err != nil {
			log.Errorf("Could not properly shut down pairing controller: %v", err)
		}

		log.Infof("Stopped pairing controller.")
	}()

	// Handle interrupt signals correctly
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt)
		sig := <-signals
		log.Info(sig)
		log.Info("Received an interrupt, stopping dispenser...")
		dispenser.Shutdown()
	}()

	// blocks until the dispenser is shut down
	err = dispenser.Run()
	if err != nil {
		return errors.Errorf("Failed running dispenser: %v", err)
	}

	// finish with no error
	return nil
}

func main() {
	// Call the "real" main in a nested manner so the defers will properly
	// be executed in the case of a graceful shutdown.
	if err := sweetdMain(); err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
		} else {
			log.WithError(err).Println("Failed running sweetd.")
		}
		os.Exit(1)
	}
}

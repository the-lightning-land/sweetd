package main

import (
	"github.com/cretz/bine/tor"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/the-lightning-land/sweetd/dispenser"
	"github.com/the-lightning-land/sweetd/machine"
	"github.com/the-lightning-land/sweetd/network"
	"github.com/the-lightning-land/sweetd/nodeman"
	"github.com/the-lightning-land/sweetd/pairing"
	"github.com/the-lightning-land/sweetd/sweetdb"
	"github.com/the-lightning-land/sweetd/sweetlog"
	"github.com/the-lightning-land/sweetd/updater"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"

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

	// The network, which acts as the core connectivity
	// provider for all other components
	var net network.Network

	if cfg.Net.Interface != "" {
		net = network.NewWpaNetwork(&network.Config{
			Interface: cfg.Net.Interface,
			Logger:    log.WithField("system", "network"),
		})

		log.Info("Created network.")
	} else {
		net = network.NewMockNetwork()

		log.Info("Created mock network.")
	}

	err = net.Start()
	if err != nil {
		return errors.Errorf("Could not start network: %v", err)
	}

	defer func() {
		err = net.Stop()
		if err != nil {
			log.Errorf("Could not properly shut down network: %v", err)
		} else {
			log.Info("Stopped network.")
		}
	}()

	// The updater which takes care of system updates
	var u updater.Updater

	switch cfg.Updater {
	case "none":
		u = updater.NewNoopUpdater()

		log.Info("Created noop updater.")
	case "mender":
		u, err = updater.NewMenderUpdater(&updater.MenderUpdaterConfig{
			Logger: log.WithField("system", "updater"),
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
		DebugWriter:     log.WithField("system", "tor").WriterLevel(log.DebugLevel),
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

	nodeman := nodeman.New(&nodeman.Config{
		NodesDataDir: filepath.Join(cfg.DataDir, "nodes"),
		DB:           sweetDB,
		LogCreator: func(node string) nodeman.Logger {
			logger := log.WithField("system", "nodeman")

			if node != "" {
				logger = logger.WithField("node", node)
			}

			return logger
		},
	})

	// central controller for everything the dispenser does
	dispenser := dispenser.NewDispenser(&dispenser.Config{
		Nodeman:  nodeman,
		Machine:  m,
		DB:       sweetDB,
		Updater:  u,
		SweetLog: sweetLog,
		Logger:   log.WithField("system", "dispenser"),
		Tor:      t,
		Network:  net,
	})

	log.Infof("Created dispenser.")

	if cfg.Pairing.Interface != "" {
		// create subsystem responsible for pairing
		pairingController, err := pairing.NewController(&pairing.Config{
			Logger:    log.WithField("system", "pairing"),
			AdapterId: cfg.Pairing.Interface,
			Dispenser: dispenser,
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
	}

	// Handle interrupt signals correctly
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt)
		sig := <-signals
		log.Info(sig)
		log.Info("Received an interrupt, stopping dispenser...")
		dispenser.Stop()
	}()

	// blocks until the dispenser is stopped
	err = dispenser.RunAndWait()
	if err != nil {
		return errors.Errorf("unable to run: %v", err)
	}

	// finish without error
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

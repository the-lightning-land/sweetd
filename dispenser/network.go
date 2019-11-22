package dispenser

import (
	"github.com/go-errors/errors"
	"github.com/the-lightning-land/sweetd/network"
	"github.com/the-lightning-land/sweetd/sweetdb"
	"sync"
)

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

func (d *Dispenser) ConnectToWifi(connection network.Connection) error {
	switch conn := connection.(type) {
	case *network.WpaPskConnection:
		d.log.Infof("Connecting to wifi %v", conn)

		err := d.network.Connect(conn)
		if err != nil {
			d.log.Errorf("Could not get Wifi networks: %v", err)
			return errors.New("Could not get Wifi networks")
		}

		err = d.SetWifiConnection(&sweetdb.WifiConnection{
			Ssid: conn.Ssid,
			Psk:  conn.Psk,
		})
		if err != nil {
			d.log.Errorf("Could not save wifi connection: %v", err)
		}
	case *network.WpaConnection:
		d.log.Infof("Connecting to wifi %v", conn)

		err := d.network.Connect(conn)
		if err != nil {
			d.log.Errorf("Could not get Wifi networks: %v", err)
			return errors.New("Could not get Wifi networks")
		}

		err = d.SetWifiConnection(&sweetdb.WifiConnection{
			Ssid: conn.Ssid,
		})
		if err != nil {
			d.log.Errorf("Could not save wifi connection: %v", err)
		}
	default:
		return errors.Errorf("unsupported connection type %T", connection)
	}

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

func (d *Dispenser) ScanWifi() (*network.ScanClient, error) {
	return d.network.Scan()
}

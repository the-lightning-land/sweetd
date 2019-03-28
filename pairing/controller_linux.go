package pairing

import (
	"encoding/json"
	"github.com/go-errors/errors"
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/linux/btmgmt"
	"github.com/muka/go-bluetooth/service"
	"github.com/the-lightning-land/sweetd/ap"
	"time"
)

const (
	// Unique UUID suffix for the candy dispenser
	uuidSuffix = "-75dd-4a0e-b688-66b7df342cc6"

	// Prefix of the candy service UUID
	candyServiceUuidPrefix = "CA00"

	// Where to expose the application
	objectName = "org.bluez"
	objectPath = "/sweet/pairing/service"

	// Local name of the application
	localName = "Candy"

	candyServiceUuid          = candyServiceUuidPrefix + "0000" + uuidSuffix
	networkAvailabilityStatus = candyServiceUuidPrefix + "CA01" + uuidSuffix
	ipAddress                 = candyServiceUuidPrefix + "CA02" + uuidSuffix
	wifiScanList              = candyServiceUuidPrefix + "CA03" + uuidSuffix
	wifiSsidString            = candyServiceUuidPrefix + "CA04" + uuidSuffix
	wifiPskString             = candyServiceUuidPrefix + "CA05" + uuidSuffix
	wifiConnectSignal         = candyServiceUuidPrefix + "CA06" + uuidSuffix
)

type Controller struct {
	log         Logger
	adapterId   string
	accessPoint ap.Ap
	app         *service.Application
	service     *service.GattService1
}

func NewController(config *Config) (*Controller, error) {
	controller := &Controller{}

	if config.Logger != nil {
		controller.log = config.Logger
	} else {
		controller.log = noopLogger{}
	}

	// Assign the device adapter id (ex. hci0)
	controller.adapterId = config.AdapterId

	// Assign the depending access point
	controller.accessPoint = config.AccessPoint

	var err error

	app := GattApp(objectName, objectPath, localName)
	service := app.Service(Primary, candyServiceUuid, Advertised)

	service.DeviceNameCharacteristic("Candy").
		UserDescriptionDescriptor("Device Name").
		PresentationDescriptor()
	service.ManufacturerNameCharacteristic("The Lightning Land").
		UserDescriptionDescriptor("Manufacturer Name").
		PresentationDescriptor()
	service.SerialNumberCharacteristic("123456789").
		UserDescriptionDescriptor("Serial Number").
		PresentationDescriptor()
	service.ModelNumberCharacteristic("moon").
		UserDescriptionDescriptor("Model Number").
		PresentationDescriptor()
	service.Characteristic(networkAvailabilityStatus, controller.readNetworkAvailabilityStatus, nil).
		UserDescriptionDescriptor("Network Availability Status")
	service.Characteristic(ipAddress, controller.readIpAddress, nil).
		UserDescriptionDescriptor("IP Address")
	service.Characteristic(wifiScanList, controller.readWifiScanList, nil).
		UserDescriptionDescriptor("Wi-Fi Scan List")
	service.Characteristic(wifiSsidString, controller.readWifiSsidString, controller.writeWifiSsidString).
		UserDescriptionDescriptor("Wi-Fi SSID")
	service.Characteristic(wifiPskString, nil, controller.writeWifiPskString).
		UserDescriptionDescriptor("Wi-Fi PSK")
	service.Characteristic(wifiConnectSignal, nil, controller.writeWifiConnectSignal).
		UserDescriptionDescriptor("Wi-Fi Connect Signal")

	controller.app, err = app.Run()
	if err != nil {
		return nil, errors.Errorf("Could not start app: %v", err)
	}

	return controller, nil
}

func (c *Controller) Start() error {
	mgmt := btmgmt.NewBtMgmt(c.adapterId)
	err := mgmt.Reset()
	if err != nil {
		return errors.Errorf("Reset %s: %v", c.adapterId, err)
	}

	// Sleep to give the device some time after the reset
	time.Sleep(time.Millisecond * 500)

	gattManager, err := api.GetGattManager(c.adapterId)
	if err != nil {
		return errors.Errorf("Get gatt manager failed: %v", err)
	}

	err = gattManager.RegisterApplication(c.app.Path(), map[string]interface{}{})
	if err != nil {
		return errors.Errorf("Register failed: %v", err)
	}

	err = c.app.StartAdvertising(c.adapterId)
	if err != nil {
		return errors.Errorf("Failed to advertise: %v", err)
	}

	return nil
}

func (c *Controller) Stop() error {
	err := c.app.StopAdvertising()
	if err != nil {
		return errors.Errorf("Could not stop advertising: %v", err)
	}

	gattManager, err := api.GetGattManager(c.adapterId)
	if err != nil {
		return errors.Errorf("Get gatt manager failed: %v", err)
	}

	err = gattManager.UnregisterApplication(c.app.Path())
	if err != nil {
		return errors.Errorf("Unregister failed: %v", err)
	}

	return nil
}

func (c *Controller) readNetworkAvailabilityStatus() ([]byte, error) {
	c.log.Infof("Reading network availability...")
	return []byte{0}, nil
}

func (c *Controller) readIpAddress() ([]byte, error) {
	c.log.Infof("Reading ip address...")
	return []byte("192.168.1.120"), nil
}

type WifiScanListItem struct {
	Ssid string `json:"ssid"`
}

func (c *Controller) readWifiScanList() ([]byte, error) {
	c.log.Infof("Reading wifi scan list...")

	networks, err := c.accessPoint.ListWifiNetworks()
	if err != nil {
		return nil, errors.Errorf("Could not get wifi scan list: %v", err)
	}

	wifiScanList := []*WifiScanListItem{}
	for _, net := range networks {
		wifiScanList = append(wifiScanList, &WifiScanListItem{
			Ssid: net.Ssid,
		})
	}

	payload, err := json.Marshal(wifiScanList)
	if err != nil {
		return nil, errors.Errorf("Could not serialize wifi scan list: %v", err)
	}

	return payload, nil
}

func (c *Controller) readWifiSsidString() ([]byte, error) {
	c.log.Infof("Reading wifi ssid...")
	return []byte("onion"), nil
}

func (c *Controller) writeWifiSsidString(value []byte) error {
	c.log.Infof("Writing wifi ssid to %v", value)
	return nil
}

func (c *Controller) writeWifiPskString(value []byte) error {
	c.log.Infof("Writing wifi psk")
	return nil
}

func (c *Controller) writeWifiConnectSignal(value []byte) error {
	c.log.Infof("Writing wifi connect signal to %v", value)
	return nil
}

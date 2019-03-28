package pairing

import (
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

	// Where to expose the application
	objectName = "org.bluez"
	objectPath = "/sweet/pairing/service"

	// Local name of the application
	localName = "Candy"

	// Device UUID
	uuid = "CA00"

	// Candy service UUID
	candyServiceUuid = "CA00"

	// Name of the device
	// utf8s
	deviceNameString = "2A00"

	// The value of this characteristic is a UTF-8 string
	// representing the name of the manufacturer of the device.
	// utf8s
	manufacturerNameString = "2A29"

	// The value of this characteristic is a variable-length UTF-8 string
	// representing the serial number for a particular instance of the device.
	// utf8s
	serialNumberString = "2A25"

	// The value of this characteristic is a UTF-8 string
	// representing the model number assigned by the device vendor.
	// utf8s
	modelNumberString = "2A24"

	// The Network Availability characteristic represents
	// if network is available or not available.
	// uint8
	// 0 No network available
	// 1 One or more networks available
	networkAvailabilityStatus = "2A3E"

	ipAddress = "CA01"

	wifiScanList = "CA02"

	wifiSsidString = "CA03"

	wifiPskString = "CA04"

	wifiConnectSignal = "CA05"
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

	app := GattApp(uuidSuffix, uuid, objectName, objectPath, localName)
	service := app.Service(Primary, candyServiceUuid, Advertised)

	service.Characteristic(deviceNameString, controller.readDeviceNameString, nil)
	service.Characteristic(manufacturerNameString, controller.readManufacturerNameString, nil)
	service.Characteristic(serialNumberString, controller.readSerialNumberString, nil)
	service.Characteristic(modelNumberString, controller.readModelNumberString, nil)
	service.Characteristic(networkAvailabilityStatus, controller.readNetworkAvailabilityStatus, nil)
	service.Characteristic(ipAddress, controller.readIpAddress, nil)
	service.Characteristic(wifiScanList, controller.readWifiScanList, nil)
	service.Characteristic(wifiSsidString, controller.readWifiSsidString, controller.writeWifiSsidString)
	service.Characteristic(wifiPskString, nil, controller.writeWifiPskString)
	service.Characteristic(wifiConnectSignal, nil, controller.writeWifiConnectSignal)

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

func (c *Controller) readDeviceNameString() ([]byte, error) {
	c.log.Infof("Reading device name...")
	return []byte("Candy"), nil
}

func (c *Controller) readManufacturerNameString() ([]byte, error) {
	c.log.Infof("Reading manufacturer name...")
	return []byte("The Lightning Land"), nil
}

func (c *Controller) readSerialNumberString() ([]byte, error) {
	c.log.Infof("Reading serial number...")
	return []byte("100000"), nil
}

func (c *Controller) readModelNumberString() ([]byte, error) {
	c.log.Infof("Reading model number...")
	return []byte("moon"), nil
}

func (c *Controller) readNetworkAvailabilityStatus() ([]byte, error) {
	c.log.Infof("Reading network availability...")
	return []byte{0}, nil
}

func (c *Controller) readIpAddress() ([]byte, error) {
	c.log.Infof("Reading ip address...")
	return []byte("192.168.1.120"), nil
}

func (c *Controller) readWifiScanList() ([]byte, error) {
	c.log.Infof("Reading wifi scan list...")
	return []byte("[{}, {}, {}]"), nil
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

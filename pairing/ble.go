package pairing

import (
	"github.com/go-errors/errors"
	"github.com/godbus/dbus/v5"
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/advertising"
	"github.com/sirupsen/logrus"
	"github.com/the-lightning-land/sweetd/network"
	"github.com/the-lightning-land/sweetd/pairing/ble"
	"time"
)

// check compliance to interface during compile time
var _ Controller = (*BLEController)(nil)

const (
	appPath = "/io/sweetbit"

	// unique uuid suffix for the candy dispenser
	uuidSuffix = "-75dd-4a0e-b688-66b7df342cc6"

	candyServiceUuid       = "ca000000" + uuidSuffix
	statusCharUuid         = "ca001000" + uuidSuffix
	scanWifiCharUuid       = "ca002000" + uuidSuffix
	discoveredWifiCharUuid = "ca003000" + uuidSuffix
	connectWifiCharUuid    = "ca004000" + uuidSuffix
	onionApiCharUuid       = "ca005000" + uuidSuffix
)

type Dispenser interface {
	ConnectWifi(network.Connection) error
	GetApiOnionID() string
	GetName() string
	ScanWifi() (*network.ScanClient, error)
}

type BLEControllerConfig struct {
	Logger    Logger
	AdapterId string
	Dispenser Dispenser
}

type BLEController struct {
	log                  Logger
	adapterId            string
	dispenser            Dispenser
	app                  *ble.GattApp
	notifyDisocveredWifi ble.Writer
	discoveredWifis      chan *network.Wifi
}

func NewController(config *BLEControllerConfig) (*BLEController, error) {
	controller := &BLEController{}

	if config.Logger != nil {
		controller.log = config.Logger
	} else {
		controller.log = noopLogger{}
	}

	controller.discoveredWifis = make(chan *network.Wifi, 10)

	// Assign the device adapter id (ex. hci0)
	controller.adapterId = config.AdapterId

	// Most pairing actions rely on functions the dispenser is providing
	controller.dispenser = config.Dispenser

	controller.app = ble.NewGattApp(
		config.AdapterId,
		appPath,
		ble.WithAppService(candyServiceUuid,
			ble.WithServiceCharacteristic(
				statusCharUuid,
				ble.WithCharacteristicReadHandler(controller.status),
				ble.WithCharacteristicUserDescriptionDescriptor("Status"),
				ble.WithCharacteristicPresentationFormatDescriptor(),
			),
			ble.WithServiceCharacteristic(
				scanWifiCharUuid,
				ble.WithCharacteristicWriteHandler(controller.scanWifi),
				ble.WithCharacteristicUserDescriptionDescriptor("Scan Wi-Fi"),
			),
			ble.WithServiceCharacteristic(
				discoveredWifiCharUuid,
				ble.WithCharacteristicWriter(&controller.notifyDisocveredWifi),
				ble.WithCharacteristicUserDescriptionDescriptor("Discovered Wi-Fi"),
			),
			ble.WithServiceCharacteristic(
				connectWifiCharUuid,
				ble.WithCharacteristicWriteHandler(controller.connectWifi),
				ble.WithCharacteristicUserDescriptionDescriptor("Connect Wi-Fi"),
			),
			ble.WithServiceCharacteristic(
				onionApiCharUuid,
				ble.WithCharacteristicReadHandler(controller.onionApi),
				ble.WithCharacteristicUserDescriptionDescriptor("Onion API"),
			),
		),
	)

	a, err := adapter.NewAdapter1FromAdapterID(controller.adapterId)
	if err != nil {
		return nil, errors.Errorf("unable to create adapter: %v", err)
	}

	err = a.SetAlias("Candy")
	if err != nil {
		return nil, errors.Errorf("unable to set alias: %v", err)
	}

	err = a.SetDiscoverable(true)
	if err != nil {
		return nil, errors.Errorf("unable to make discoverable: %v", err)
	}

	err = a.SetDiscoverableTimeout(0)
	if err != nil {
		return nil, errors.Errorf("unable to set discoverable timeout: %v", err)
	}

	err = a.SetPowered(true)
	if err != nil {
		return nil, errors.Errorf("unable to set powered: %v", err)
	}

	adv := &advertising.LEAdvertisement1Properties{
		Type:      advertising.AdvertisementTypePeripheral,
		LocalName: "Candy",
		ServiceUUIDs: []string{
			candyServiceUuid,
		},
	}

	advManagerPath := dbus.ObjectPath("/org/bluez/hci0/app/adv")

	_, err = api.ExposeAdvertisement(controller.adapterId, adv, 0)
	if err != nil {
		return nil, errors.Errorf("unable to expose advertisement: %v", err)
	}

	advManager, err := advertising.NewLEAdvertisingManager1FromAdapterID(config.AdapterId)
	if err != nil {
		logrus.Fatalf("unable to create advertisement manager: %v", err)
	}

	err = advManager.RegisterAdvertisement(advManagerPath, map[string]interface{}{})
	if err != nil {
		logrus.Fatalf("unable to register advertisement manager: %v", err)
	}

	return controller, nil
}

func (c *BLEController) Advertise() error {
	return nil
}

func (c *BLEController) Start() error {
	err := c.app.Start()
	if err != nil {
		return errors.Errorf("unable to start app: %v", err)
	}

	go func() {
		for {
			wifi, ok := <-c.discoveredWifis
			if !ok {
				break
			}

			err := c.notifyDisocveredWifi([]byte(wifi.Ssid))
			if err != nil {
				c.log.Errorf("unable to write discovered wifi: %v", err)
			}

			time.Sleep(100 * time.Millisecond)
		}
	}()

	c.discoveredWifis <- &network.Wifi{Ssid: "jo"}

	return nil
}

func (c *BLEController) Stop() error {
	err := c.app.Stop()
	if err != nil {
		return errors.Errorf("unable to stop app: %v", err)
	}

	return nil
}

func (c *BLEController) status() ([]byte, error) {
	return []byte("works"), nil
}

func (c *BLEController) scanWifi(value []byte) error {
	client, err := c.dispenser.ScanWifi()
	if err != nil {
		return errors.Errorf("unable to scan: %v", err)
	}

	for {
		// this channel will close when the scan is finished
		wifi, ok := <-client.Wifis
		if !ok {
			break
		}

		c.discoveredWifis <- wifi
	}

	return nil
}

func (c *BLEController) connectWifi(value []byte) error {
	return nil
}

func (c *BLEController) onionApi() ([]byte, error) {
	return []byte(c.dispenser.GetApiOnionID()), nil
}

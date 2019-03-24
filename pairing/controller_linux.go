package pairing

import (
	"github.com/go-errors/errors"
	"github.com/godbus/dbus"
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile"
	"github.com/muka/go-bluetooth/service"
	"github.com/the-lightning-land/sweetd/ap"
	"time"
)

const (
	objectName                    = "org.bluez"
	objectPath                    = dbus.ObjectPath("/sweet/pairing/service")
	localName                     = "Candy"
	advertisedOptional            = false
	deviceUuid                    = "3e2194ca-0eb4-4f9b-917d-4fa52d65d3c2"
	serviceUuid                   = "09e1cdf3-f1a8-4bd0-9451-d6abc007a660"
	getNameCharacteristicUuid     = "ce2cba89-75dd-4a0e-b688-66b7df342cc6"
	getSerialNoCharacteristicUuid = "ebde779d-0fe9-4f84-ba47-07080b8356ea"
	getIpCharacteristicUuid       = "5abdac9d-7643-4fad-9ff9-2fbfc2d52edd"
	getWifiSsidCharacteristicUuid = "cb912938-d3cc-4524-85c8-4839f7b544cb"
	scanWifiCharacteristicUuid    = "718d3f42-f9b1-48bd-9ac1-34b35e66afa9"
	setWifiSsidCharacteristicUuid = "982c536f-3610-4234-8a3a-b55194a4a188"
	setWifiPskCharacteristicUuid  = "b765a814-ed6b-4a1e-85ab-3ec3a343c2d2"
	connectWifiCharacteristicUuid = "4960b1e2-0bb8-4c7a-a560-9f5172bb7e25"
)

type Characteristics struct {
	getName     *service.GattCharacteristic1
	getSerialNo *service.GattCharacteristic1
	getIp       *service.GattCharacteristic1
	getWifiSsid *service.GattCharacteristic1
	scanWifi    *service.GattCharacteristic1
	setWifiSsid *service.GattCharacteristic1
	setWifiPsk  *service.GattCharacteristic1
	connectWifi *service.GattCharacteristic1
}

type Controller struct {
	log             Logger
	adapterId       string
	accessPoint     ap.Ap
	app             *service.Application
	service         *service.GattService1
	characteristics *Characteristics
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

	controller.app, err = service.NewApplication(&service.ApplicationConfig{
		UUIDSuffix: "-0000-1000-8000-00805F9B34FB",
		UUID:       "1235",
		ObjectName: objectName,
		ObjectPath: objectPath,
		LocalName:  localName,
		//ReadFunc:   controller.handleRead,
		//WriteFunc:  controller.handleWrite,
	})
	if err != nil {
		return nil, errors.Errorf("Failed to initialize app: %v", err)
	}

	controller.log.Infof("Created application")
	time.Sleep(time.Second)

	err = controller.app.Run()
	if err != nil {
		return nil, errors.Errorf("Failed to run: %v", err)
	}

	controller.log.Infof("Ran application")
	time.Sleep(time.Second)

	controller.service, err = controller.app.CreateService(&profile.GattService1Properties{
		Primary: true,
		UUID:    controller.app.GenerateUUID("4444"),
	}, advertisedOptional)
	if err != nil {
		return nil, errors.Errorf("Failed to create service: %v", err)
	}

	controller.log.Infof("Created service")
	time.Sleep(time.Second)

	err = controller.app.AddService(controller.service)
	if err != nil {
		return nil, errors.Errorf("Failed to add service: %v", err)
	}

	controller.log.Infof("Added service")
	time.Sleep(time.Second)

	controller.characteristics = &Characteristics{}

	controller.characteristics.getName, err = controller.service.CreateCharacteristic(&profile.GattCharacteristic1Properties{
		UUID:      controller.app.GenerateUUID("4444"),
		// Notifying: true,
		Flags: []string{
			bluez.FlagCharacteristicRead,
			bluez.FlagCharacteristicWrite,
			bluez.FlagCharacteristicNotify,
		},
	})
	if err != nil {
		return nil, errors.Errorf("Failed to create char: %v", err)
	}

	err = controller.service.AddCharacteristic(controller.characteristics.getName)
	if err != nil {
		return nil, errors.Errorf("Failed to add char: %v", err)
	}

	desc, err := controller.characteristics.getName.CreateDescriptor(&profile.GattDescriptor1Properties{
		UUID: controller.app.GenerateUUID("4444"),
		Flags: []string{
			bluez.FlagDescriptorRead,
			bluez.FlagDescriptorWrite,
		},
	})
	if err != nil {
		return nil, errors.Errorf("Failed to create char: %v", err)
	}

	err = controller.characteristics.getName.AddDescriptor(desc)
	if err != nil {
		return nil, errors.Errorf("Failed to add desc: %v", err)
	}

	return controller, nil
}

func (c *Controller) Start() error {
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

	c.log.Infof("Started advertising")

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

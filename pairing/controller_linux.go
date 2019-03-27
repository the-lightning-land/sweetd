package pairing

import (
	"github.com/go-errors/errors"
	"github.com/godbus/dbus"
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile"
	"github.com/muka/go-bluetooth/service"
	"github.com/the-lightning-land/sweetd/ap"
)

const (
	uuidSuffix                    = "75dd-4a0e-b688-66b7df342cc6"
	objectName                    = "org.bluez"
	objectPath                    = dbus.ObjectPath("/sweet/pairing/service")
	localName                     = "Candy"
	advertisedOptional            = true
	uuid                          = "AAAA"
	serviceUuid                   = "1111"
	getNameCharacteristicUuid     = "1111"
	getSerialNoCharacteristicUuid = "2222"
	getIpCharacteristicUuid       = "3333"
	getWifiSsidCharacteristicUuid = "4444"
	scanWifiCharacteristicUuid    = "5555"
	setWifiSsidCharacteristicUuid = "6666"
	setWifiPskCharacteristicUuid  = "7777"
	connectWifiCharacteristicUuid = "8888"
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
		UUIDSuffix: uuidSuffix,
		UUID:       uuid,
		ObjectName: objectName,
		ObjectPath: objectPath,
		LocalName:  localName,
		ReadFunc:   controller.handleRead,
		WriteFunc:  controller.handleWrite,
	})
	if err != nil {
		return nil, errors.Errorf("Failed to initialize app: %v", err)
	}

	err = controller.app.Run()
	if err != nil {
		return nil, errors.Errorf("Failed to run: %v", err)
	}

	controller.service, err = controller.app.CreateService(&profile.GattService1Properties{
		Primary: true,
		UUID:    serviceUuid,
	}, advertisedOptional)
	if err != nil {
		return nil, errors.Errorf("Failed to create service: %v", err)
	}

	err = controller.app.AddService(controller.service)
	if err != nil {
		return nil, errors.Errorf("Failed to add service: %v", err)
	}

	controller.characteristics = &Characteristics{}

	controller.characteristics.getName, err = controller.service.CreateCharacteristic(&profile.GattCharacteristic1Properties{
		UUID: getNameCharacteristicUuid,
		// Notifying: true,
		Flags: []string{
			bluez.FlagCharacteristicRead,
			bluez.FlagCharacteristicWrite,
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
		UUID: getNameCharacteristicUuid,
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

func (c *Controller) handleRead(app *service.Application, serviceUuid string, charUuid string) ([]byte, error) {
	c.log.Infof("Reading %v", charUuid)
	return []byte("ewfewf"), nil
}

func (c *Controller) handleWrite(app *service.Application, serviceUuid string, charUuid string, value []byte) error {
	c.log.Infof("Writing %v = %v", charUuid, value)
	return nil
}

func (c *Controller) Start() error {
	// adapterID := 0

	//mgmt := btmgmt.NewBtMgmt(c.adapterId)
	//err := mgmt.Reset()
	//if err != nil {
	//	return errors.Errorf("Reset %s: %v", c.adapterId, err)
	//}
	//
	//time.Sleep(time.Millisecond * 500)

	//err = linux.Up(adapterID)
	//if err != nil {
	//	return errors.Errorf("Failed to start device hci%d: %v", adapterID, err)
	//}
	//
	//c.log.Infof("Upped")

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

	//api.On("data", emitter.NewCallback(func(ev emitter.Event) {
	//	print(ev.GetData())
	//	print(ev.GetName())
	//}))

	// TODO: where to move this?
	//err = api.On("data", emitter.NewCallback(func(ev emitter.Event) {
	//	print(ev.GetData())
	//	print(ev.GetName())
	//}))
	//if err != nil {
	//	c.log.Errorf("Could not subscribe: %v", err)
	//}
	//
	//err = c.characteristics.getName.StartNotify()
	//if err != nil {
	//	c.log.Errorf("Could not notify: %v", err)
	//}

	//ticker := time.NewTicker(5 * time.Second)
	//for {
	//	select {
	//	case <-ticker.C:
	//		val, err := c.characteristics.getName.ReadValue(map[string]interface{}{})
	//		c.log.Infof("Value is: %s", val)
	//		print(err)
	//	}
	//}

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

// Convenient methods for populating Gatt services,
// characteristics and descriptors

package pairing

import (
	"github.com/go-errors/errors"
	"github.com/godbus/dbus"
	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile"
	"github.com/muka/go-bluetooth/service"
)

type PrimaryType bool

const Primary = PrimaryType(true)
const Secondary = PrimaryType(false)

type AdvertisedType bool

const Advertised = AdvertisedType(true)
const AdvertisedOptional = AdvertisedType(false)

type HandleRead = func() ([]byte, error)
type HandleWrite = func(value []byte) error

type gattApp struct {
	app           *service.Application
	err           error
	readHandlers  map[string]HandleRead
	writeHandlers map[string]HandleWrite
}

type gattService struct {
	*gattApp
	service *service.GattService1
}

func GattApp(uuidSuffix string, uuid string, objectName string, objectPath string, localName string) *gattApp {
	a := &gattApp{}
	var err error

	a.readHandlers = make(map[string]HandleRead)
	a.writeHandlers = make(map[string]HandleWrite)

	a.app, err = service.NewApplication(&service.ApplicationConfig{
		UUIDSuffix: uuidSuffix,
		UUID:       uuid,
		ObjectName: objectName,
		ObjectPath: dbus.ObjectPath(objectPath),
		LocalName:  localName,
		ReadFunc:   a.handleRead,
		WriteFunc:  a.handleWrite,
	})
	if err != nil {
		return &gattApp{
			err: errors.Errorf("Could not create app: %v", err),
		}
	}

	return a
}

func (a *gattApp) handleRead(app *service.Application, serviceUuid string, characteristicUuid string) ([]byte, error) {
	if readHandler, ok := a.readHandlers[characteristicUuid]; ok {
		return readHandler()
	}

	return nil, errors.Errorf("No read handler found in service %v for characteristic %v", serviceUuid, characteristicUuid)
}

func (a *gattApp) handleWrite(app *service.Application, serviceUuid string, characteristicUuid string, value []byte) error {
	if writeHandler, ok := a.writeHandlers[characteristicUuid]; ok {
		return writeHandler(value)
	}

	return errors.Errorf("No write handler found in service %v for characteristic %v", serviceUuid, characteristicUuid)
}

func (a *gattApp) Run() (*service.Application, error) {
	if a.err != nil {
		return nil, a.err
	}

	err := a.app.Run()
	if err != nil {
		return nil, errors.Errorf("Could not run app: %v", err)
	}

	return a.app, nil
}

func (a *gattApp) Service(primaryType PrimaryType, uuid string, advertised AdvertisedType) *gattService {
	if a.err != nil {
		return &gattService{gattApp: a}
	}

	svc, err := a.app.CreateService(&profile.GattService1Properties{
		Primary: bool(primaryType),
		UUID:    a.app.GenerateUUID(uuid),
	}, bool(advertised))

	if err != nil {
		a.err = errors.Errorf("Failed to create service: %v", err)
		return &gattService{gattApp: a}
	}

	err = a.app.AddService(svc)
	if err != nil {
		a.err = errors.Errorf("Failed to add service: %v", err)
		return &gattService{gattApp: a}
	}

	return &gattService{
		gattApp: a,
		service: svc,
	}
}

func (s *gattService) Characteristic(uuid string, read HandleRead, write HandleWrite) *gattService {
	if s.err != nil {
		return s
	}

	characteristicUuid := s.app.GenerateUUID(uuid)

	var inferredFlags []string

	if read != nil {
		inferredFlags = append(inferredFlags, bluez.FlagCharacteristicRead)

		// TODO: Mapping by characteristic UUID only makes this work for one service
		s.readHandlers[characteristicUuid] = read
	}

	if write != nil {
		inferredFlags = append(inferredFlags, bluez.FlagCharacteristicWrite)

		// TODO: Mapping by characteristic UUID only makes this work for one service
		s.writeHandlers[characteristicUuid] = write
	}

	characteristic, err := s.service.CreateCharacteristic(&profile.GattCharacteristic1Properties{
		UUID:  characteristicUuid,
		Flags: inferredFlags,
	})

	if err != nil {
		s.err = errors.Errorf("Failed to create characteristic: %v", err)
		return s
	}

	err = s.service.AddCharacteristic(characteristic)
	if err != nil {
		s.err = errors.Errorf("Failed to add characteristic: %v", err)
		return s
	}

	return s
}

package wpa

import (
	"github.com/go-errors/errors"
	"github.com/godbus/dbus/v5"
)

type Interface struct {
	wpa *Wpa
	obj dbus.BusObject
}

func (i *Interface) Scan() error {
	call := i.obj.Call("fi.w1.wpa_supplicant1.Interface.Scan", 0, map[string]interface{}{
		"Type": "active",
	})
	if call.Err != nil {
		return errors.Errorf("could not find scan: %v", call.Err)
	}

	return nil
}

type BSSAddedClient struct {
	BSSAdded <-chan *BSS
	Cancel   func()
}

func (i *Interface) BSSAdded() (*BSSAddedClient, error) {
	bssChan := make(chan *BSS)
	signalChan := make(chan *dbus.Signal)

	client := &BSSAddedClient{
		BSSAdded: bssChan,
		Cancel: func() {
			i.wpa.conn.RemoveSignal(signalChan)

			_ = i.wpa.conn.BusObject().RemoveMatchSignal("fi.w1.wpa_supplicant1.Interface", "BSSAdded", dbus.WithMatchObjectPath(i.obj.Path()))

			close(signalChan)
			close(bssChan)
		},
	}

	go func() {
		i.wpa.conn.Signal(signalChan)

		for {
			select {
			case signal, ok := <-signalChan:
				if ok {
					return
				}

				if signal.Name == "fi.w1.wpa_supplicant1.Interface.BSSAdded" && signal.Path == i.obj.Path() {
					bssChan <- &BSS{
						obj: i.wpa.conn.Object("fi.w1.wpa_supplicant1", signal.Path),
					}
				}
			}
		}
	}()

	call := i.wpa.conn.BusObject().AddMatchSignal("fi.w1.wpa_supplicant1.Interface", "BSSAdded", dbus.WithMatchObjectPath(i.obj.Path()))
	if call.Err != nil {
		return nil, errors.Errorf("could not add signal: %v", call.Err)
	}

	return client, nil
}

type ScanDoneClient struct {
	ScanDone <-chan bool
	Cancel   func()
}

func (i *Interface) ScanDone() (*ScanDoneClient, error) {
	changeChan := make(chan bool)
	signalChan := make(chan *dbus.Signal)

	client := &ScanDoneClient{
		ScanDone: changeChan,
		Cancel: func() {
			i.wpa.conn.RemoveSignal(signalChan)

			_ = i.wpa.conn.BusObject().RemoveMatchSignal("fi.w1.wpa_supplicant1.Interface", "ScanDone", dbus.WithMatchObjectPath(i.obj.Path()))

			close(signalChan)
			close(changeChan)
		},
	}

	go func() {
		i.wpa.conn.Signal(signalChan)

		for {
			select {
			case signal, ok := <-signalChan:
				if !ok {
					return
				}

				if signal.Name == "fi.w1.wpa_supplicant1.Interface.ScanDone" && signal.Path == i.obj.Path() {
					println(signal.Name, signal.Body[0].(bool))

					changeChan <- signal.Body[0].(bool)
				}
			}
		}
	}()

	call := i.wpa.conn.BusObject().AddMatchSignal("fi.w1.wpa_supplicant1.Interface", "ScanDone", dbus.WithMatchObjectPath(i.obj.Path()))
	if call.Err != nil {
		return nil, errors.Errorf("could not add signal: %v", call.Err)
	}

	return client, nil
}

func (i *Interface) PropertiesChanged() error {
	call := i.wpa.conn.BusObject().AddMatchSignal("fi.w1.wpa_supplicant1.Interface", "PropertiesChanged", dbus.WithMatchObjectPath(i.obj.Path()))
	if call.Err != nil {
		return errors.Errorf("could not add signal: %v", call.Err)
	}

	return nil
}

func (i *Interface) BSSs() ([]*BSS, error) {
	v, err := i.obj.GetProperty("fi.w1.wpa_supplicant1.Interface.BSSs")
	if err != nil {
		return nil, errors.Errorf("could not get bsss: %v", err)
	}

	objectPaths, ok := v.Value().([]dbus.ObjectPath)
	if !ok {
		return nil, errors.Errorf("could not convert result: %v", err)
	}

	var bsss []*BSS

	for _, objectPath := range objectPaths {
		bsss = append(bsss, &BSS{
			obj: i.wpa.conn.Object("fi.w1.wpa_supplicant1", objectPath),
		})
	}

	return bsss, nil
}

func (i *Interface) AddNetwork(ssid string, psk string) (*Network, error) {
	args := map[string]interface{}{}

	if psk != "" {
		args["ssid"] = ssid
		args["psk"] = psk
	} else {
		args["ssid"] = ssid
		args["key_mgmt"] = "NONE"
	}

	call := i.obj.Call("fi.w1.wpa_supplicant1.Interface.AddNetwork", 0, args)
	if call.Err != nil {
		return nil, errors.Errorf("could not call: %v", call.Err)
	}

	var objPath dbus.ObjectPath
	err := call.Store(&objPath)
	if err != nil {
		return nil, errors.Errorf("could not store value: %v", err)
	}

	netObj := i.wpa.conn.Object("fi.w1.wpa_supplicant1", objPath)

	return &Network{
		wpa: i.wpa,
		obj: netObj,
	}, nil
}

func (i *Interface) RemoveNetwork(net *Network) error {
	call := i.obj.Call("fi.w1.wpa_supplicant1.Interface.RemoveNetwork", 0, net.obj.Path())
	if call.Err != nil {
		return errors.Errorf("could not remove network: %v", call.Err)
	}

	return nil
}

func (i *Interface) RemoveAllNetworks(net *Network) error {
	call := i.obj.Call("fi.w1.wpa_supplicant1.Interface.RemoveAllNetworks", 0)
	if call.Err != nil {
		return errors.Errorf("could not remove all networks: %v", call.Err)
	}

	return nil
}

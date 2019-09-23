package wpa

import (
	"encoding/hex"
	"github.com/go-errors/errors"
	"github.com/godbus/dbus/v5"
)

type BSS struct {
	obj dbus.BusObject
}

func (b *BSS) String() string {
	return string(b.obj.Path())
}

type Bss struct {
	Ssid  string
	Bssid string
}

func (b *BSS) GetAll() (*Bss, error) {
	call := b.obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, "fi.w1.wpa_supplicant1.BSS")
	if call.Err != nil {
		return nil, errors.Errorf("could not get all properties: %v", call.Err)
	}

	props, ok := call.Body[0].(map[string]dbus.Variant)
	if !ok {
		return nil, errors.Errorf("could convert output")
	}

	bss := Bss{}

	if val, ok := props["SSID"]; ok {
		if ssid, ok := val.Value().([]byte); ok {
			bss.Ssid = string(ssid)
		} else {
			return nil, errors.Errorf("could not convert SSID to string: %v", val)
		}
	} else {
		return nil, errors.Errorf("mandatory property SSID was missing")
	}

	if val, ok := props["BSSID"]; ok {
		if bssid, ok := val.Value().([]byte); ok {
			bss.Bssid = hex.EncodeToString(bssid)
		} else {
			return nil, errors.Errorf("could not convert BSSID to string: %v", val)
		}
	} else {
		return nil, errors.Errorf("mandatory property BSSID was missing")
	}

	return &bss, nil
}

package wpa

import "github.com/godbus/dbus/v5"

type Network struct {
	wpa *Wpa
	obj dbus.BusObject
}

func (n *Network) String() string {
	return string(n.obj.Path())
}

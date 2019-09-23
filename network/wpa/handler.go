package wpa

import "github.com/godbus/dbus/v5"

type wpaSignalHandler struct {
	*Wpa
}

var _ dbus.SignalHandler = (*wpaSignalHandler)(nil)

func (n wpaSignalHandler) DeliverSignal(iface, name string, signal *dbus.Signal) {
	n.deliverSignal(iface, name, signal)
}

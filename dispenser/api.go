package dispenser

import "net"

type Api interface {
	SetDispenser(d *Dispenser)
	Serve(l net.Listener) error
}

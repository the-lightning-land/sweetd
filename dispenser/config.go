package dispenser

import (
	"github.com/cretz/bine/tor"
	"github.com/the-lightning-land/sweetd/ap"
	"github.com/the-lightning-land/sweetd/machine"
	"github.com/the-lightning-land/sweetd/pos"
	"github.com/the-lightning-land/sweetd/sweetdb"
	"github.com/the-lightning-land/sweetd/sweetlog"
	"github.com/the-lightning-land/sweetd/updater"
)

type Config struct {
	Machine     machine.Machine
	AccessPoint ap.Ap
	DB          *sweetdb.DB
	MemoPrefix  string
	Updater     updater.Updater
	Pos         *pos.Pos
	SweetLog    *sweetlog.SweetLog
	Logger      Logger
	Tor         *tor.Tor
	Api         Api
}

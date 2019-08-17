package dispenser

import "github.com/the-lightning-land/sweetd/updater"

func (d *Dispenser) Update(url string) (*updater.Update, error) {
	return d.updater.StartUpdate(url)
}

func (d *Dispenser) SubscribeUpdate(id string) (*updater.UpdateClient, error) {
	return d.updater.SubscribeUpdate(id)
}

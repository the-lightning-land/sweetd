package updater

type UpdateClient struct {
	Update     chan *Update
	Id         uint32
	cancelChan chan struct{}
	updateId   string
	updater    Updater
}

func (c *UpdateClient) Cancel() error {
	return c.updater.unsubscribeUpdate(c)
}

package updater

import "time"

type State = string

const StateStarted State = "started"
const StateCancelled State = "cancelled"
const StateFailed State = "failed"
const StateInstalled State = "installed"
const StateRejected State = "rejected"
const StateCompleted State = "completed"

type Update struct {
	Id           string
	Started      time.Time
	Url          string
	State        State
	Progress     uint8
	ShouldReboot bool
	ShouldCommit bool
}

type Updater interface {
	GetVersion() (string, error)
	StartUpdate(url string) (*Update, error)
	CancelUpdate() error
	SubscribeUpdate(id string) (*UpdateClient, error)
	UnsubscribeUpdate(client *UpdateClient) error
}

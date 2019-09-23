package connectivity

import (
	"context"
)

type State int

const (
	Offline State = iota
	Online
)

func (s State) String() string {
	switch s {
	case Offline:
		return "OFFLINE"
	case Online:
		return "ONLINE"
	default:
		return "INVALID STATE"
	}
}

type Reporter interface {
	CurrentState() State
	WaitForStateChange(context.Context, State) bool
}

type SomeReporter struct {
}

func NewReporter() Reporter {
	return &SomeReporter{}
}

func (r *SomeReporter) CurrentState() State {
	return Offline
}

func (r *SomeReporter) WaitForStateChange(ctx context.Context, state State) bool {
	return false
}

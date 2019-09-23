package network

type Status struct {
	connected bool
}

func (s *Status) Connected() bool {
	return s.connected
}

type Connection interface{}

type Network interface {
	Start() error
	Stop() error
	Status() *Status
	Connect(Connection) error
	Scan() error
	Subscribe() *Client
	deleteClient(uint32)
}

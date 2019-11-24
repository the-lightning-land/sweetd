package network

type Status struct {
	connected bool
}

func (s *Status) Connected() bool {
	return s.connected
}

type Connection interface{}

type WpaPskConnection struct {
	Ssid string
	Psk  string
}

type WpaConnection struct {
	Ssid string
}

type EncryptionType uint8

const (
	EncryptionPersonal EncryptionType = iota
	EncryptionEnterprise
	EncryptionNone
)

type Wifi struct {
	Ssid       string
	Encryption EncryptionType
}

type ScanClient struct {
	Wifis  <-chan *Wifi
	Cancel func()
}

type Network interface {
	Start() error
	Stop() error
	Status() *Status
	Connect(Connection) error
	Scan() (*ScanClient, error)
	Subscribe() *Client
	deleteClient(uint32)
}

package node

type Invoice struct {
	paymentRequest string
}

type Node interface {
	Start() error
	Stop() error
	GetInvoice(rHash string) (*Invoice, error)
	AddInvoice() (*Invoice, error)
}

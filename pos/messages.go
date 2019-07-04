package pos

type messageType string

const (
  subscribeInvoiceStatus messageType = "subscribe_invoice_status"
  changedInvoiceStatus               = "changed_invoice_status"
  addInvoice                         = "add_invoice"
  createdInvoice                     = "created_invoice"
)

type envelope struct {
  messageType messageType `json:"type"`
  message     interface{} `json:"message"`
}

var messageTypeHandlers = map[messageType]func() interface{}{
  subscribeInvoiceStatus: func() interface{} { return &subscribeInvoiceStatusMessage{} },
  changedInvoiceStatus:   func() interface{} { return &invoiceStatusChangedMessage{} },
  addInvoice:             func() interface{} { return &addInvoiceMessage{} },
  createdInvoice:         func() interface{} { return &createdInvoiceMessage{} },
}

type subscribeInvoiceStatusMessage struct {
  messageType string `json`
  rHash       string `json:"r_hash"`
}

type invoiceStatusChangedMessage struct {
  rHash   string `json:"r_hash"`
  settled bool   `json:"settled"`
}

type addInvoiceMessage struct {
}

type createdInvoiceMessage struct {
  rHash   string `json:"r_hash"`
  invoice string `json:invoice`
}

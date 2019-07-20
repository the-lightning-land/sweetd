package node

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"github.com/go-errors/errors"
	"github.com/lightningnetwork/lnd/lnrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	"sync"
	"time"
)

var (
	beginCertificateBlock = []byte("-----BEGIN CERTIFICATE-----\n")
	endCertificateBlock   = []byte("\n-----END CERTIFICATE-----")
)

type LndNodeConfig struct {
	Uri           string
	CertBytes     []byte
	MacaroonBytes []byte
	Logger        Logger
}

type LndNode struct {
	uri                  string
	tlsCredentials       credentials.TransportCredentials
	macaroonMetadata     metadata.MD
	conn                 *grpc.ClientConn
	client               lnrpc.LightningClient
	logger               Logger
	invoicesClients      map[uint32]*InvoicesClient
	invoicesClientMtx    sync.Mutex
	nextInvoicesClientID uint32
}

// Compile time check for protocol compatibility
var _ Node = (*LndNode)(nil)

func NewLndNode(config *LndNodeConfig) (*LndNode, error) {
	cert := x509.NewCertPool()
	fullCertBytes := append(beginCertificateBlock, config.CertBytes...)
	fullCertBytes = append(fullCertBytes, endCertificateBlock...)

	if ok := cert.AppendCertsFromPEM(fullCertBytes); !ok {
		return nil, errors.New("could not parse tls cert")
	}

	tlsCredentials := credentials.NewClientTLSFromCert(cert, "")

	hexMacaroon := hex.EncodeToString(config.MacaroonBytes)
	macaroonMetadata := metadata.Pairs("macaroon", hexMacaroon)

	return &LndNode{
		uri:              config.Uri,
		tlsCredentials:   tlsCredentials,
		macaroonMetadata: macaroonMetadata,
		logger:           config.Logger,
		invoicesClients:  make(map[uint32]*InvoicesClient),
	}, nil
}

func (r *LndNode) Start() error {
	var err error
	r.conn, err = grpc.Dial(r.uri, grpc.WithTransportCredentials(r.tlsCredentials))
	if err != nil {
		return errors.Errorf("Could not connect to lightning node: %v", err)
	}

	r.client = lnrpc.NewLightningClient(r.conn)

	go r.run()

	return nil
}

func (r *LndNode) run() {
	ctx := context.Background()
	ctx = metadata.NewOutgoingContext(ctx, r.macaroonMetadata)

	invoices, err := r.client.SubscribeInvoices(ctx, &lnrpc.InvoiceSubscription{})
	if err != nil {
		r.logger.Errorf("Could not subscribe to invoices: %v", err)
	}

	for {
		invoice, err := invoices.Recv()
		if err == io.EOF {
			r.logger.Errorf("Got EOF from invoices stream: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if err != nil {
			errStatus, ok := status.FromError(err)
			if !ok {
				r.logger.Errorf("Could not get status from err: %v", err)
			}

			if errStatus.Code() == 1 {
				r.logger.Infof("Stopping invoice listener")
				break
			} else if err != nil {
				r.logger.Errorf("Failed receiving subscription items: %v", err)
				break
			}
		}

		for _, client := range r.invoicesClients {
			client.Invoices <- &Invoice{
				RHash:          hex.EncodeToString(invoice.RHash),
				PaymentRequest: invoice.PaymentRequest,
				MSat:           invoice.Value,
				Settled:        invoice.Settled,
				Memo:           invoice.Memo,
			}
		}
	}
}

func (r *LndNode) Stop() error {
	err := r.conn.Close()
	if err != nil {
		return errors.Errorf("Could not close connection: %v", err)
	}

	return nil
}

func (r *LndNode) GetInvoice(rHash string) (*Invoice, error) {
	if r.client == nil {
		return nil, errors.Errorf("Node not started")
	}

	ctx := context.Background()
	ctx = metadata.NewOutgoingContext(ctx, r.macaroonMetadata)

	res, err := r.client.LookupInvoice(ctx, &lnrpc.PaymentHash{
		RHashStr: rHash,
	})
	if err != nil {
		return nil, errors.Errorf("Could not find invoice: %v", err)
	}

	return &Invoice{
		Settled:        res.Settled,
		RHash:          hex.EncodeToString(res.RHash),
		PaymentRequest: res.PaymentRequest,
		Memo:           res.Memo,
		MSat:           res.Value,
	}, nil
}

func (r *LndNode) AddInvoice(req *InvoiceRequest) (*Invoice, error) {
	if r.client == nil {
		return nil, errors.Errorf("Node not started")
	}

	ctx := context.Background()
	ctx = metadata.NewOutgoingContext(ctx, r.macaroonMetadata)

	res, err := r.client.AddInvoice(ctx, &lnrpc.Invoice{
		Memo:  "Candy for 8 satoshis",
		Value: 8,
	})
	if err != nil {
		return nil, errors.Errorf("Could not add invoice: %v", err)
	}

	return &Invoice{
		Settled:        false,
		RHash:          hex.EncodeToString(res.RHash),
		PaymentRequest: res.PaymentRequest,
		Memo:           req.Memo,
		MSat:           req.MSat,
	}, nil
}

func (r *LndNode) SubscribeInvoices() (*InvoicesClient, error) {
	client := &InvoicesClient{
		Invoices:   make(chan *Invoice),
		cancelChan: make(chan struct{}),
		node:       r,
	}

	r.invoicesClientMtx.Lock()
	client.Id = r.nextInvoicesClientID
	r.nextInvoicesClientID++
	r.invoicesClientMtx.Unlock()

	r.invoicesClients[client.Id] = client

	return client, nil
}

func (r *LndNode) unsubscribeInvoices(client *InvoicesClient) error {
	delete(r.invoicesClients, client.Id)
	close(client.cancelChan)
	return nil
}

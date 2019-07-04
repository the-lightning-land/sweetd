package node

import (
	"crypto/x509"
	"encoding/hex"
	"github.com/go-errors/errors"
	"github.com/lightningnetwork/lnd/lnrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
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
	uri              string
	tlsCredentials   credentials.TransportCredentials
	macaroonMetadata metadata.MD
	conn             *grpc.ClientConn
	client           lnrpc.LightningClient
	logger           Logger
}

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
	}, nil
}

func (r *LndNode) Start() error {
	var err error
	r.conn, err = grpc.Dial(r.uri, grpc.WithTransportCredentials(r.tlsCredentials))
	if err != nil {
		return errors.Errorf("Could not connect to lightning node: %v", err)
	}

	r.client = lnrpc.NewLightningClient(r.conn)

	// go r.run()

	return nil
}

//func (r *LndNode) run() {
//	ctx := context.Background()
//	ctx = metadata.NewOutgoingContext(ctx, r.macaroonMetadata)
//
//	invoices, err := r.client.SubscribeInvoices(ctx, &lnrpc.InvoiceSubscription{})
//	if err != nil {
//		r.logger.Errorf("Could not subscribe to invoices: %v", err)
//	}
//
//	for {
//		invoice, err := invoices.Recv()
//		if err == io.EOF {
//			r.logger.Errorf("Got EOF from invoices stream: %v", err)
//			time.Sleep(1 * time.Second)
//			continue
//		}
//
//		if err != nil {
//			errStatus, ok := status.FromError(err)
//			if !ok {
//				r.logger.Errorf("Could not get status from err: %v", err)
//			}
//
//			if errStatus.Code() == 1 {
//				r.logger.Infof("Stopping invoice listener")
//				break
//			} else if err != nil {
//				r.logger.Errorf("Failed receiving subscription items: %v", err)
//				break
//			}
//		}
//
//		if invoice.Settled {
//			if d.memoPrefix == "" ||
//				(d.memoPrefix != "" && strings.HasPrefix(invoice.Memo, d.memoPrefix)) {
//				log.Debugf("Received settled payment of %v sat", invoice.Value)
//				d.payments <- invoice
//			} else {
//				log.Infof("Received payment with memo %s but memo prefix is %s.", invoice.Memo, d.memoPrefix)
//			}
//		} else {
//			log.Debugf("Generated invoice of %v sat", invoice.Value)
//		}
//	}
//}

func (r *LndNode) Stop() error {
	err := r.conn.Close()
	if err != nil {
		return errors.Errorf("Could not close connection: %v", err)
	}

	return nil
}

func (r *LndNode) GetInvoice(rHash string) (*Invoice, error) {
	return nil, nil
}

func (r *LndNode) AddInvoice() (*Invoice, error) {
	return nil, nil
}

func (r *LndNode) Subscribe() error {
	return nil
}

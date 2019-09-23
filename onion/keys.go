package onion

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/asn1"
	"encoding/base32"
	"github.com/go-errors/errors"
)

type Version int

const (
	V2 Version = iota
	V3  // TODO(davidknezic): support V3
)

func GeneratePrivateKey(v Version) (*rsa.PrivateKey, error) {
	if v != V2 {
		return nil, errors.New("only V2 supported for now")
	}

	// Generate a V2 RSA 1024 bit key
	return rsa.GenerateKey(rand.Reader, 1024)
}

// standard base32 encoding with lowercase characters
var base32encoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567")

func computeV2IDFromV2PublicKey(key crypto.PublicKey) string {
	derbytes, _ := asn1.Marshal(key)

	// 1. Let H = H(PK).
	hash := sha1.New()
	hash.Write(derbytes)
	sum := hash.Sum(nil)

	// 2. Let H' = the first 80 bits of H, considering each octet from
	//    most significant bit to least significant bit.
	sum = sum[:10]

	// 3. Generate a 16-character encoding of H', using base32 as defined
	//    in RFC 4648.
	var buf32 bytes.Buffer
	b32enc := base32.NewEncoder(base32encoding, &buf32)
	_, _ = b32enc.Write(sum)
	_ = b32enc.Close()

	return buf32.String()
}

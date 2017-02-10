package tx

import (
	"github.com/pkg/errors"
	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
	lightclient "github.com/tendermint/light-client"
)

// MultiSig lets us wrap arbitrary data with a go-crypto signature
//
// TODO: rethink how we want to integrate this with KeyStore so it makes
// more sense (particularly the verify method)
type MultiSig struct {
	data []byte
	sigs []signed
}

type signed struct {
	sig    crypto.Signature
	pubkey crypto.PubKey
}

func NewMulti(data []byte) *MultiSig {
	return &MultiSig{data: data}
}

func LoadMulti(serialized []byte) (*MultiSig, error) {
	var s MultiSig
	err := wire.ReadBinaryBytes(serialized, &s)
	return &s, err
}

// assertSignable is just to make sure we stay in sync with the Signable interface
func (s *MultiSig) assertSignable() lightclient.Signable {
	return s
}

// Bytes returns the original data passed into `NewSig`
func (s *MultiSig) Bytes() []byte {
	return s.data
}

// Sign will add a signature and pubkey.
//
// Depending on the Signable, one may be able to call this multiple times for multisig
// Returns error if called with invalid data or too many times
func (s *MultiSig) Sign(pubkey crypto.PubKey, sig crypto.Signature) error {
	if pubkey == nil || sig == nil {
		return errors.New("Signature or Key missing")
	}

	// set the value once we are happy
	x := signed{sig, pubkey}
	s.sigs = append(s.sigs, x)
	return nil
}

// Signers will return the public key(s) that signed if the signature
// is valid, or an error if there is any issue with the signature,
// including if there are no signatures
func (s *MultiSig) Signers() ([]crypto.PubKey, error) {
	if len(s.sigs) == 0 {
		return nil, errors.New("Never signed")
	}

	keys := make([]crypto.PubKey, len(s.sigs))
	for i := range s.sigs {
		ms := s.sigs[i]
		if !ms.pubkey.VerifyBytes(s.data, ms.sig) {
			return nil, errors.Errorf("Signature %d doesn't match (key: %X)", i, ms.pubkey.Bytes())
		}
		keys[i] = ms.pubkey
	}

	return keys, nil
}

// SignBytes serializes the Sig to send it to a tendermint app.
// It returns an error if the Sig was never Signed.
func (s *MultiSig) SignBytes() ([]byte, error) {
	if len(s.sigs) == 0 {
		return nil, errors.New("Never signed")
	}
	return wire.BinaryBytes(wrapper{s}), nil
}

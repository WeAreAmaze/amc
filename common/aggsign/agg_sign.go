package aggsign

import (
	"github.com/amazechain/amc/common/crypto/bls"
	"github.com/amazechain/amc/common/types"
)

var SigChannel = make(chan AggSign, 10)

type AggSign struct {
	Number    uint64          `json:"number"`
	StateRoot types.Hash      `json:"stateRoot"`
	Sign      types.Signature `json:"sign"`
	Address   types.Address   `json:"address"`
	PublicKey types.PublicKey `json:"-"`
}

func (s *AggSign) Check(root types.Hash) bool {
	if s.StateRoot != root {
		return false
	}
	sig, err := bls.SignatureFromBytes(s.Sign[:])
	if nil != err {
		return false
	}

	pub, err := bls.PublicKeyFromBytes(s.PublicKey[:])
	if nil != err {
		return false
	}
	return sig.Verify(pub, s.StateRoot[:])
}

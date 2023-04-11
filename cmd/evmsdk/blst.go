package evmsdk

import (
	"encoding/hex"

	"github.com/amazechain/amc/common/crypto/bls"
)

func BlsSign(privKey, msg string) (interface{}, error) {
	msgBytes, err := hex.DecodeString(msg)
	if err != nil {
		return nil, err
	}
	privKeyBytes, err := hex.DecodeString(privKey)
	if err != nil {
		return nil, err
	}
	arr := [32]byte{}
	copy(arr[:], privKeyBytes[:])
	sk, err := bls.SecretKeyFromRandom32Byte(arr)
	if err != nil {
		return nil, err
	}
	resBytes := sk.Sign(msgBytes).Marshal()
	return hex.EncodeToString(resBytes), nil
}

func BlsPublicKey(privKey string) (interface{}, error) {
	privKeyBytes, err := hex.DecodeString(privKey)
	if err != nil {
		return nil, err
	}
	arr := [32]byte{}
	copy(arr[:], privKeyBytes[:])
	sk, err := bls.SecretKeyFromRandom32Byte(arr)
	if err != nil {
		return nil, err
	}
	resBytes := sk.PublicKey().Marshal()
	return hex.EncodeToString(resBytes), nil
}

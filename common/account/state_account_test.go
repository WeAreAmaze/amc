package account

import (
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/types"
	"github.com/holiman/uint256"
	"testing"
)

func TestEmptyAccount2(t *testing.T) {
	encodedAccount := StateAccount{}

	b := make([]byte, encodedAccount.EncodingLengthForStorage())
	encodedAccount.EncodeForStorage(b)

	var decodedAccount StateAccount
	if err := decodedAccount.DecodeForStorage(b); err != nil {
		t.Fatal("cant decode the account", err, encodedAccount)
	}
}

func TestAccountEncodeWithCode(t *testing.T) {
	a := StateAccount{
		Initialised: true,
		Nonce:       2,
		Balance:     *new(uint256.Int).SetUint64(1000),
		Root:        types.HexToHash("0000000000000000000000000000000000000000000000000000000000000021"),
		CodeHash:    types.BytesToHash(crypto.Keccak256([]byte{1, 2, 3})),
		Incarnation: 4,
	}

	encodedLen := a.EncodingLengthForStorage()
	encodedAccount := make([]byte, encodedLen)
	a.EncodeForStorage(encodedAccount)

	var decodedAccount StateAccount
	if err := decodedAccount.DecodeForStorage(encodedAccount); err != nil {
		t.Fatal("cant decode the account", err, encodedAccount)
	}

	isAccountsEqual(t, a, decodedAccount)
}

func isAccountsEqual(t *testing.T, src, dst StateAccount) {
	if dst.Initialised != src.Initialised {
		t.Fatal("cant decode the account Initialised", src.Initialised, dst.Initialised)
	}

	if dst.CodeHash != src.CodeHash {
		t.Fatal("cant decode the account CodeHash", src.CodeHash, dst.CodeHash)
	}

	if dst.Balance.Cmp(&src.Balance) != 0 {
		t.Fatal("cant decode the account Balance", src.Balance, dst.Balance)
	}

	if dst.Nonce != src.Nonce {
		t.Fatal("cant decode the account Nonce", src.Nonce, dst.Nonce)
	}
	if dst.Incarnation != src.Incarnation {
		t.Fatal("cant decode the account Version", src.Incarnation, dst.Incarnation)
	}
}

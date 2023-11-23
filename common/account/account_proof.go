package account

import (
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/common/types"
)

// Result structs for GetProof
type AccProofResult struct {
	Address      types.Address     `json:"address"`
	AccountProof []hexutil.Bytes   `json:"accountProof"`
	Balance      *hexutil.Big      `json:"balance"`
	CodeHash     types.Hash        `json:"codeHash"`
	Nonce        hexutil.Uint64    `json:"nonce"`
	StorageHash  types.Hash        `json:"storageHash"`
	StorageProof []StorProofResult `json:"storageProof"`
}
type StorProofResult struct {
	Key   types.Hash      `json:"key"`
	Value *hexutil.Big    `json:"value"`
	Proof []hexutil.Bytes `json:"proof"`
}

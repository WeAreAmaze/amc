// Copyright 2023 The AmazeChain Authors
// This file is part of the AmazeChain library.
//
// The AmazeChain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The AmazeChain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the AmazeChain library. If not, see <http://www.gnu.org/licenses/>.

package amtdeposit

import (
	"bytes"
	"embed"
	"github.com/amazechain/amc/accounts/abi"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"
	"math/big"
)

//go:embed abi.json
var abiJson embed.FS
var contractAbi abi.ABI
var depositSignature = crypto.Keccak256Hash([]byte("DepositEvent(bytes,uint256,bytes)"))
var withdrawnSignature = crypto.Keccak256Hash([]byte("WithdrawnEvent(uint256)"))

func init() {
	var (
		depositAbiCode []byte
		err            error
	)
	if depositAbiCode, err = abiJson.ReadFile("abi.json"); err != nil {
		panic("Could not open abi.json")
	}

	if contractAbi, err = abi.JSON(bytes.NewReader(depositAbiCode)); err != nil {
		panic("unable to parse AMT deposit contract abi")
	}
}

type Contract struct {
}

func (Contract) WithdrawnSignature() types.Hash {
	return withdrawnSignature
}

func (Contract) DepositSignature() types.Hash {
	return depositSignature
}

func (Contract) UnpackLogData(data []byte) (publicKey []byte, signature []byte, depositAmount *uint256.Int, err error) {
	var (
		unpackedLogs []interface{}
		overflow     bool
	)
	//
	if unpackedLogs, err = contractAbi.Unpack("DepositEvent", data); err != nil {
		err = errors.Wrap(err, "unable to unpack logs")
		return
	}
	//
	if depositAmount, overflow = uint256.FromBig(unpackedLogs[1].(*big.Int)); overflow {
		err = errors.New("unable to unpack amount")
		return
	}
	publicKey, signature = unpackedLogs[0].([]byte), unpackedLogs[2].([]byte)
	log.Debug("unpacked DepositEvent Logs", "publicKey", hexutil.Encode(unpackedLogs[0].([]byte)), "signature", hexutil.Encode(unpackedLogs[2].([]byte)), "message", hexutil.Encode(depositAmount.Bytes()))
	return
}

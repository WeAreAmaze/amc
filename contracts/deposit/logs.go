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

package deposit

import (
	"bytes"
	"github.com/amazechain/amc/accounts/abi"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/log"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"
	"math/big"
)

// UnpackDepositLogData unpacks the data from a deposit log using the ABI decoder.
func UnpackDepositLogData(data []byte) (pubkey []byte, amount *uint256.Int, signature []byte, err error) {
	reader := bytes.NewReader(depositAbiCode)
	contractAbi, err := abi.JSON(reader)
	if err != nil {
		return nil, uint256.NewInt(0), nil, errors.Wrap(err, "unable to generate contract abi")
	}

	unpackedLogs, err := contractAbi.Unpack("DepositEvent", data)
	if err != nil {
		return nil, uint256.NewInt(0), nil, errors.Wrap(err, "unable to unpack logs")
	}
	amount, overflow := uint256.FromBig(unpackedLogs[1].(*big.Int))
	if overflow {
		return nil, uint256.NewInt(0), nil, errors.New("unable to unpack amount")
	}

	log.Debug("unpacked DepositEvent Logs", "publicKey", hexutil.Encode(unpackedLogs[0].([]byte)), "signature", hexutil.Encode(unpackedLogs[2].([]byte)), "message", hexutil.Encode(amount.Bytes()))

	return unpackedLogs[0].([]byte), amount, unpackedLogs[2].([]byte), nil
}

// UnpackWithdrawnLogData unpacks the data from a deposit log using the ABI decoder.
func UnpackWithdrawnLogData(data []byte) (amount *uint256.Int, err error) {
	reader := bytes.NewReader(depositAbiCode)
	contractAbi, err := abi.JSON(reader)
	if err != nil {
		return nil, errors.Wrap(err, "unable to generate contract abi")
	}

	unpackedLogs, err := contractAbi.Unpack("WithdrawnEvent", data)
	if err != nil {
		return nil, errors.Wrap(err, "unable to unpack logs")
	}
	amount, overflow := uint256.FromBig(unpackedLogs[0].(*big.Int))
	if overflow {
		return nil, errors.New("unable to unpack amount")
	}

	log.Debug("unpacked WithdrawnEvent Logs", "message", hexutil.Encode(amount.Bytes()))

	return amount, nil
}

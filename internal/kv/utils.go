// Copyright 2022 The AmazeChain Authors
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

package kv

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/amazechain/amc/common/types"
)

var ErrKeyNotFound = errors.New("db: key not found")

func Walk(c Cursor, startkey []byte, fixedbits int, walker func(k, v []byte) (bool, error)) error {
	fixedbytes, mask := Bytesmask(fixedbits)
	k, v, err := c.Seek(startkey)
	if err != nil {
		return err
	}
	for k != nil && len(k) >= fixedbytes && (fixedbits == 0 || bytes.Equal(k[:fixedbytes-1], startkey[:fixedbytes-1]) && (k[fixedbytes-1]&mask) == (startkey[fixedbytes-1]&mask)) {
		goOn, err := walker(k, v)
		if err != nil {
			return err
		}
		if !goOn {
			break
		}
		k, v, err = c.Next()
		if err != nil {
			return err
		}
	}
	return nil
}

// todo: return TEVM code and use it
func GetHasTEVM(db Has) func(contractHash types.Hash) (bool, error) {
	contractsWithTEVM := map[types.Hash]struct{}{}
	var ok bool

	return func(contractHash types.Hash) (bool, error) {
		if contractHash == (types.Hash{}) {
			return false, nil
		}

		if _, ok = contractsWithTEVM[contractHash]; ok {
			return true, nil
		}

		ok, err := db.Has(ContractTEVMCode, contractHash.Bytes())
		if err != nil && !errors.Is(err, ErrKeyNotFound) {
			return false, fmt.Errorf("can't check TEVM bucket by contract %q hash: %w",
				contractHash.String(), err)
		}

		if errors.Is(err, ErrKeyNotFound) {
			return false, nil
		}

		if ok {
			contractsWithTEVM[contractHash] = struct{}{}
		}

		return true, nil
	}
}

func Bytesmask(fixedbits int) (fixedbytes int, mask byte) {
	fixedbytes = (fixedbits + 7) / 8
	shiftbits := fixedbits & 7
	mask = byte(0xff)
	if shiftbits != 0 {
		mask = 0xff << (8 - shiftbits)
	}
	return fixedbytes, mask
}

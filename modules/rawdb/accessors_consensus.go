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

package rawdb

import (
	"bytes"
	consensus_pb "github.com/amazechain/amc/api/protocol/consensus_proto"
	"github.com/amazechain/amc/common/db"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/utils"
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/crypto"
)

func StoreSigners(db db.IDatabase, data []byte) error {
	w, err := db.OpenWriter(signersDB)
	if err != nil {
		return err
	}

	key := bytes.NewBufferString(signersDB)

	err = w.Put(key.Bytes(), data)

	return err
}

func StoreRecentSigners(db db.IDatabase, recentSigner []byte) error {
	w, err := db.OpenWriter(recentSignersDB)
	if err != nil {
		return err
	}

	key := bytes.NewBufferString(recentSignersDB)
	return w.Put(key.Bytes(), recentSigner)
}

func RecentSigners(db db.IDatabase) ([]byte, error) {
	r, err := db.OpenReader(recentSignersDB)
	if err != nil {
		return nil, err
	}

	key := bytes.NewBufferString(recentSignersDB)
	return r.Get(key.Bytes())

}

func Signers(db db.IDatabase) (map[types.Address]crypto.PubKey, error) {
	r, err := db.OpenReader(signersDB)
	if err != nil {
		return nil, err
	}

	key := bytes.NewBufferString(signersDB)

	v, err := r.Get(key.Bytes())
	if err != nil {
		return nil, err
	}

	var pbSigners consensus_pb.PBSigners
	err = proto.Unmarshal(v, &pbSigners)
	if err != nil {
		return nil, err
	}
	result := make(map[types.Address]crypto.PubKey)
	//log.Infof("pb singers %v", pbSigners)
	for _, miner := range pbSigners.Signer {
		public, err := utils.StringToPublic(miner.Public)
		if err != nil {
			return nil, err
		}
		result[miner.Address] = public
	}

	return result, nil
}

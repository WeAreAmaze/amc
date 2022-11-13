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

package lmdb

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/amazechain/amc/common/db"
	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/utils"
	"github.com/c2h5oh/datasize"
	"github.com/torquem-ch/mdbx-go/mdbx"
)

var (
	_lmdb Lmdb
)

type Lmdb struct {
	*mdbx.Env
	config *conf.DatabaseConfig

	ctx    context.Context
	cancel context.CancelFunc

	once    sync.Once
	running bool
	wg      sync.WaitGroup
	mu      sync.RWMutex

	mDBI map[string]*DBI
}

func NewLMDB(c context.Context, nodeConfig *conf.NodeConfig, config *conf.DatabaseConfig) (*Lmdb, error) { //db.IDatabase
	if _lmdb.running {
		return &_lmdb, nil
	}
	env, err := mdbx.NewEnv()
	if err != nil {
		log.Errorf("failed to create lmdb, err %v", err)
	}

	if config.Debug {
		if err := env.SetDebug(mdbx.LogLvlDebug, mdbx.DbgDoNotChange, mdbx.LoggerDoNotChange); err != nil {
			log.Errorf("failed to set lmdb with deubg, err: %v", err)
			return nil, err
		}
	}

	if err = env.SetOption(mdbx.OptMaxDB, config.MaxDB); err != nil {
		log.Errorf("failed to set max db, err: %v", err)
		return nil, err
	}

	if err = env.SetOption(mdbx.OptMaxReaders, config.MaxReaders); err != nil {
		log.Errorf("failed to set max reader, err: %v", err)
		return nil, err
	}

	if err = env.SetGeometry(-1, -1, int(3*datasize.TB), int(2*datasize.GB), -1, 4*1024); err != nil {
		log.Errorf("failed to set geometry, err: %v", err)
		return nil, err
	}
	var file string
	//todo how deal with windows?
	if strings.HasSuffix(config.DBPath, "/") {
		file = fmt.Sprintf("%s/%s%s", nodeConfig.DataDir, config.DBPath, config.DBName)
	} else {
		file = fmt.Sprintf("%s/%s/%s", nodeConfig.DataDir, config.DBPath, config.DBName)
	}

	if !utils.Exists(file) {
		if err := utils.MkdirAll(file, os.ModePerm); err != nil {
			return nil, err
		}
	}

	if err := env.Open(file, 0, os.ModePerm); err != nil {
		if mdbx.IsNotExist(err) {
			log.Warnf("failed to open db %s, path not exist, err: %v", file, err)
			if err := utils.MkdirAll(file, 0666); err != nil {
				return nil, err
			}
		} else {
			log.Errorf("failed to open db %s, err: %v", file, err)
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(c)
	_lmdb = Lmdb{
		Env:     env,
		config:  config,
		ctx:     ctx,
		cancel:  cancel,
		running: true,
		mDBI:    make(map[string]*DBI),
	}

	return &_lmdb, nil
}

func (m *Lmdb) OpenReader(dbName string) (reader db.IDatabaseReader, err error) {
	return m.openDBI(dbName)
}

func (m *Lmdb) OpenWriter(dbName string) (writer db.IDatabaseWriter, err error) {
	return m.openDBI(dbName)
}

func (m *Lmdb) Open(dbName string) (rw db.IDatabaseWriterReader, err error) {
	return m.openDBI(dbName)
}

/*
	Snapshot:
*/
func (m *Lmdb) Snapshot() (db.ISnapshot, error) {
	return newSnapshot(m.ctx, nil, m.Env)
}

func (m *Lmdb) openDBI(dbName string) (rw db.IDatabaseWriterReader, err error) {
	m.mu.RLock()
	if dbi, ok := m.mDBI[dbName]; ok {
		m.mu.RUnlock()
		return dbi, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	dbi, err := newDBI(m.ctx, m.Env, dbName)
	if err != nil {
		return nil, err
	}

	m.mDBI[dbName] = dbi
	return dbi, nil
}

func (m *Lmdb) Close() (err error) {
	m.once.Do(func() {
		m.running = false
		m.cancel()
		m.Env.Close()
	})
	return
}

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

package zap

import (
	"testing"

	"go.uber.org/zap"

	"github.com/amazechain/amc/log"
)

func TestLogger(t *testing.T) {
	zaplog, err := zap.NewProduction()
	if err != nil {
		t.Fatal(err)
	}
	logger := NewLogger(zaplog)

	defer func() { _ = logger.Sync() }()

	zlog := log.NewHelper(logger)
	//
	zlog.Debugw("log", "debug")
	//zlog.Infow("log", "info")
	//zlog.Warnw("log", "warn")
	//zlog.Errorw("log", "error")

	log.SetLogger(logger)

	log.Debugf("aa %d %s %d", 1, "2", 3)
}

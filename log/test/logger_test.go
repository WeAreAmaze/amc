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

package test

import (
	"github.com/amazechain/amc/log"
	"strings"
	"testing"
)

func TestLogger(t *testing.T) {
	//Init(nil)
	//sugared := Logger.Sugar()

	sugarLogger.Info("test info", "param 1", "param 2", "param 3")
	log.Debugf("test debugf %s, %s, %s", "param 1", "param 2", "param 3")
	sugarLogger.Debugf("test debugf %s, %s, %s", "param 1", "param 2", "param 3")
	log.Error("test error", "param 1", "param 2", "param 3")
	log.Warn("test warn", "param 1", "param 2", "param 3")
	DPanic("test panic", "param 1", "param 2", "param 3")
}

func TestError(t *testing.T) {
	s := "./aaalog"
	i := strings.LastIndex(s, ".")
	t.Log(i)
	t.Log(s[:5])
}

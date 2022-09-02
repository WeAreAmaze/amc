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

package log

import (
	"context"
	"testing"
)

func TestValue(t *testing.T) {
	logger := DefaultLogger
	logger = With(logger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	_ = logger.Log(LevelInfo, "msg", "helloworld")

	logger = DefaultLogger
	logger = With(logger)
	_ = logger.Log(LevelDebug, "msg", "helloworld")

	var v1 interface{}
	got := Value(context.Background(), v1)
	if got != v1 {
		t.Errorf("Value() = %v, want %v", got, v1)
	}
	var v2 Valuer = func(ctx context.Context) interface{} {
		return 3
	}
	got = Value(context.Background(), v2)
	res := got.(int)
	if res != 3 {
		t.Errorf("Value() = %v, want %v", res, 3)
	}
}

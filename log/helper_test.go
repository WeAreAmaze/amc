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
	"io"
	"os"
	"testing"
)

func TestHelper(t *testing.T) {
	logger := With(DefaultLogger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	log := NewHelper(logger)

	log.Log(LevelDebug, "msg", "test debug")
	log.Debug("test debug")
	log.Debugf("test %s", "debug")
	log.Debugw("log", "test debug")

	log.Warn("test warn")
	log.Warnf("test %s", "warn")
	log.Warnw("log", "test warn")
}

func TestHelperWithMsgKey(t *testing.T) {
	logger := With(DefaultLogger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	log := NewHelper(logger, WithMessageKey("message"))
	log.Debugf("test %s", "debug")
	log.Debugw("log", "test debug")
}

func TestHelperLevel(t *testing.T) {
	log := NewHelper(DefaultLogger)
	log.Debug("test debug")
	log.Info("test info")
	log.Infof("test %s", "info")
	log.Warn("test warn")
	log.Error("test error")
	log.Errorf("test %s", "error")
	log.Errorw("log", "test error")
}

func BenchmarkHelperPrint(b *testing.B) {
	log := NewHelper(NewStdLogger(io.Discard))
	for i := 0; i < b.N; i++ {
		log.Debug("test")
	}
}

func BenchmarkHelperPrintf(b *testing.B) {
	log := NewHelper(NewStdLogger(io.Discard))
	for i := 0; i < b.N; i++ {
		log.Debugf("%s", "test")
	}
}

func BenchmarkHelperPrintw(b *testing.B) {
	log := NewHelper(NewStdLogger(io.Discard))
	for i := 0; i < b.N; i++ {
		log.Debugw("key", "value")
	}
}

type traceKey struct{}

func TestContext(t *testing.T) {
	logger := With(NewStdLogger(os.Stdout),
		"trace", Trace(),
	)
	log := NewHelper(logger)
	ctx := context.WithValue(context.Background(), traceKey{}, "2233")
	log.WithContext(ctx).Info("got trace!")
}

func Trace() Valuer {
	return func(ctx context.Context) interface{} {
		s, ok := ctx.Value(traceKey{}).(string)
		if !ok {
			return nil
		}
		return s
	}
}

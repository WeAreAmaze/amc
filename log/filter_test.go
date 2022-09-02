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
	"bytes"
	"io"
	"testing"
)

func TestFilterAll(t *testing.T) {
	logger := With(DefaultLogger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	log := NewHelper(NewFilter(logger,
		FilterLevel(LevelDebug),
		FilterKey("username"),
		FilterValue("hello"),
		FilterFunc(testFilterFunc),
	))
	log.Log(LevelDebug, "msg", "test debug")
	log.Info("hello")
	log.Infow("password", "123456")
	log.Infow("username", "kratos")
	log.Warn("warn log")
}

func TestFilterLevel(t *testing.T) {
	logger := With(DefaultLogger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	log := NewHelper(NewFilter(NewFilter(logger, FilterLevel(LevelWarn))))
	log.Log(LevelDebug, "msg1", "te1st debug")
	log.Debug("test debug")
	log.Debugf("test %s", "debug")
	log.Debugw("log", "test debug")
	log.Warn("warn log")
}

func TestFilterCaller(t *testing.T) {
	logger := With(DefaultLogger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	log := NewFilter(logger)
	_ = log.Log(LevelDebug, "msg1", "te1st debug")
	logHelper := NewHelper(NewFilter(logger))
	logHelper.Log(LevelDebug, "msg1", "te1st debug")
}

func TestFilterKey(t *testing.T) {
	logger := With(DefaultLogger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	log := NewHelper(NewFilter(logger, FilterKey("password")))
	log.Debugw("password", "123456")
}

func TestFilterValue(t *testing.T) {
	logger := With(DefaultLogger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	log := NewHelper(NewFilter(logger, FilterValue("debug")))
	log.Debugf("test %s", "debug")
}

func TestFilterFunc(t *testing.T) {
	logger := With(DefaultLogger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	log := NewHelper(NewFilter(logger, FilterFunc(testFilterFunc)))
	log.Debug("debug level")
	log.Infow("password", "123456")
}

func BenchmarkFilterKey(b *testing.B) {
	log := NewHelper(NewFilter(NewStdLogger(io.Discard), FilterKey("password")))
	for i := 0; i < b.N; i++ {
		log.Infow("password", "123456")
	}
}

func BenchmarkFilterValue(b *testing.B) {
	log := NewHelper(NewFilter(NewStdLogger(io.Discard), FilterValue("password")))
	for i := 0; i < b.N; i++ {
		log.Infow("password")
	}
}

func BenchmarkFilterFunc(b *testing.B) {
	log := NewHelper(NewFilter(NewStdLogger(io.Discard), FilterFunc(testFilterFunc)))
	for i := 0; i < b.N; i++ {
		log.Info("password", "123456")
	}
}

func testFilterFunc(level Level, keyvals ...interface{}) bool {
	if level == LevelWarn {
		return true
	}
	for i := 0; i < len(keyvals); i++ {
		if keyvals[i] == "password" {
			keyvals[i+1] = fuzzyStr
		}
	}
	return false
}

func TestFilterFuncWitchLoggerPrefix(t *testing.T) {
	buf := new(bytes.Buffer)
	tests := []struct {
		logger Logger
		want   string
	}{
		{
			logger: NewFilter(With(NewStdLogger(buf), "caller", "caller", "prefix", "whaterver"), FilterFunc(testFilterFuncWithLoggerPrefix)),
			want:   "",
		},
		{
			logger: NewFilter(With(NewStdLogger(buf), "caller", "caller"), FilterFunc(testFilterFuncWithLoggerPrefix)),
			want:   "INFO caller=caller msg=msg\n",
		},
	}

	for _, tt := range tests {
		err := tt.logger.Log(LevelInfo, "msg", "msg")
		if err != nil {
			t.Fatal("err should be nil")
		}
		got := buf.String()
		if got != tt.want {
			t.Fatalf("filter should catch prefix, want %s, got %s.", tt.want, got)
		}
		buf.Reset()
	}
}

func testFilterFuncWithLoggerPrefix(level Level, keyvals ...interface{}) bool {
	if level == LevelWarn {
		return true
	}
	for i := 0; i < len(keyvals); i += 2 {
		if keyvals[i] == "prefix" {
			return true
		}
	}
	return false
}

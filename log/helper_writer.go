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

import "io"

type writerWrapper struct {
	helper *Helper
	level  Level
}

type WriterOptionFn func(w *writerWrapper)

// WithWriteLevel set writerWrapper level.
func WithWriterLevel(level Level) WriterOptionFn {
	return func(w *writerWrapper) {
		w.level = level
	}
}

// WithWriteMessageKey set writerWrapper helper message key.
func WithWriteMessageKey(key string) WriterOptionFn {
	return func(w *writerWrapper) {
		w.helper.msgKey = key
	}
}

// NewWriter return a writer wrapper.
func NewWriter(logger Logger, opts ...WriterOptionFn) io.Writer {
	ww := &writerWrapper{
		helper: NewHelper(logger, WithMessageKey(DefaultMessageKey)),
		level:  LevelInfo, // default level
	}
	for _, opt := range opts {
		opt(ww)
	}
	return ww
}

func (ww *writerWrapper) Write(p []byte) (int, error) {
	ww.helper.Log(ww.level, ww.helper.msgKey, string(p))
	return 0, nil
}

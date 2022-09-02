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
	"strings"
	"testing"
)

func TestWriterWrapper(t *testing.T) {
	var buf bytes.Buffer
	logger := NewStdLogger(&buf)
	content := "ThisIsSomeTestLogMessage"
	testCases := []struct {
		w                io.Writer
		acceptLevel      Level
		acceptMessageKey string
	}{
		{
			w:                NewWriter(logger),
			acceptLevel:      LevelInfo, // default level
			acceptMessageKey: DefaultMessageKey,
		},
		{
			w:                NewWriter(logger, WithWriterLevel(LevelDebug)),
			acceptLevel:      LevelDebug,
			acceptMessageKey: DefaultMessageKey,
		},
		{
			w:                NewWriter(logger, WithWriteMessageKey("XxXxX")),
			acceptLevel:      LevelInfo, // default level
			acceptMessageKey: "XxXxX",
		},
		{
			w:                NewWriter(logger, WithWriterLevel(LevelError), WithWriteMessageKey("XxXxX")),
			acceptLevel:      LevelError,
			acceptMessageKey: "XxXxX",
		},
	}
	for _, tc := range testCases {
		_, err := tc.w.Write([]byte(content))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), tc.acceptLevel.String()) {
			t.Errorf("expected level: %s, got: %s", tc.acceptLevel, buf.String())
		}
		if !strings.Contains(buf.String(), tc.acceptMessageKey) {
			t.Errorf("expected message key: %s, got: %s", tc.acceptMessageKey, buf.String())
		}
	}
}

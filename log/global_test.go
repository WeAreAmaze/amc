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
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestGlobalLog(t *testing.T) {
	buffer := &bytes.Buffer{}
	SetLogger(NewStdLogger(buffer))

	testCases := []struct {
		level   Level
		content []interface{}
	}{
		{
			LevelDebug,
			[]interface{}{"test debug"},
		},
		{
			LevelInfo,
			[]interface{}{"test info"},
		},
		{
			LevelInfo,
			[]interface{}{"test %s", "info"},
		},
		{
			LevelWarn,
			[]interface{}{"test warn"},
		},
		{
			LevelError,
			[]interface{}{"test error"},
		},
		{
			LevelError,
			[]interface{}{"test %s", "error"},
		},
	}

	expected := []string{}
	for _, tc := range testCases {
		msg := fmt.Sprintf(tc.content[0].(string), tc.content[1:]...)
		switch tc.level {
		case LevelDebug:
			Debugf(tc.content[0].(string), tc.content[1:]...)
			expected = append(expected, fmt.Sprintf("%s msg=%s", "DEBUG", msg))
		case LevelInfo:
			Infof(tc.content[0].(string), tc.content[1:]...)
			expected = append(expected, fmt.Sprintf("%s msg=%s", "INFO", msg))
		case LevelWarn:
			Warnf(tc.content[0].(string), tc.content[1:]...)
			expected = append(expected, fmt.Sprintf("%s msg=%s", "WARN", msg))
		case LevelError:
			Errorf(tc.content[0].(string), tc.content[1:]...)
			expected = append(expected, fmt.Sprintf("%s msg=%s", "ERROR", msg))
		}
	}
	expected = append(expected, "")

	t.Logf("Content: %s", buffer.String())
	if buffer.String() != strings.Join(expected, "\n") {
		t.Errorf("Expected: %s, got: %s", strings.Join(expected, "\n"), buffer.String())
	}
}

func TestGlobalLogUpdate(t *testing.T) {
	l := &loggerAppliance{}
	l.SetLogger(NewStdLogger(os.Stdout))
	LOG := NewHelper(l)
	LOG.Info("Log to stdout")

	buffer := &bytes.Buffer{}
	l.SetLogger(NewStdLogger(buffer))
	LOG.Info("Log to buffer")

	expected := "INFO msg=Log to buffer\n"
	if buffer.String() != expected {
		t.Errorf("Expected: %s, got: %s", expected, buffer.String())
	}
}

func TestGolbalContext(t *testing.T) {
	buffer := &bytes.Buffer{}
	SetLogger(NewStdLogger(buffer))
	Context(context.Background()).Infof("111")
	if buffer.String() != "INFO msg=111\n" {
		t.Errorf("Expected:%s, got:%s", "INFO msg=111", buffer.String())
	}
}

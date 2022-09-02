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

import "testing"

func TestLevel_String(t *testing.T) {
	tests := []struct {
		name string
		l    Level
		want string
	}{
		{
			name: "DEBUG",
			l:    LevelDebug,
			want: "DEBUG",
		},
		{
			name: "INFO",
			l:    LevelInfo,
			want: "INFO",
		},
		{
			name: "WARN",
			l:    LevelWarn,
			want: "WARN",
		},
		{
			name: "ERROR",
			l:    LevelError,
			want: "ERROR",
		},
		{
			name: "FATAL",
			l:    LevelFatal,
			want: "FATAL",
		},
		{
			name: "other",
			l:    10,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want Level
	}{
		{
			name: "DEBUG",
			want: LevelDebug,
			s:    "DEBUG",
		},
		{
			name: "INFO",
			want: LevelInfo,
			s:    "INFO",
		},
		{
			name: "WARN",
			want: LevelWarn,
			s:    "WARN",
		},
		{
			name: "ERROR",
			want: LevelError,
			s:    "ERROR",
		},
		{
			name: "FATAL",
			want: LevelFatal,
			s:    "FATAL",
		},
		{
			name: "other",
			want: LevelInfo,
			s:    "other",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseLevel(tt.s); got != tt.want {
				t.Errorf("ParseLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

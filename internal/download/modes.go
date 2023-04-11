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

package download

import "fmt"

// SyncMode represents the synchronisation mode of the downloader.
// It is a uint32 as it is used with atomic operations.
type SyncMode uint32

const (
	FullSync SyncMode = iota
	SnapSync
	LightSync
	HeaderSync
)

func (mode SyncMode) IsValid() bool {
	return mode >= FullSync && mode <= HeaderSync
}

// String implements the stringer interface.
func (mode SyncMode) String() string {
	switch mode {
	case FullSync:
		return "full"
	case SnapSync:
		return "snap"
	case LightSync:
		return "light"
	case HeaderSync:
		return "header"
	default:
		return "unknown"
	}
}

func (mode SyncMode) MarshalText() ([]byte, error) {
	switch mode {
	case FullSync:
		return []byte("full"), nil
	case SnapSync:
		return []byte("snap"), nil
	case LightSync:
		return []byte("light"), nil
	case HeaderSync:
		return []byte("header"), nil
	default:
		return nil, fmt.Errorf("unknown sync mode %d", mode)
	}
}

func (mode *SyncMode) UnmarshalText(text []byte) error {
	switch string(text) {
	case "full":
		*mode = FullSync
	case "snap":
		*mode = SnapSync
	case "light":
		*mode = LightSync
	case "header":
		*mode = HeaderSync
	default:
		return fmt.Errorf(`unknown sync mode %q, want "full", "snap" or "light"`, text)
	}
	return nil
}

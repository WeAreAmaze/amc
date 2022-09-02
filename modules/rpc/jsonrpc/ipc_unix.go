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

//go:build darwin || dragonfly || freebsd || linux || nacl || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

package jsonrpc

import (
	"context"
	"fmt"
	"github.com/amazechain/amc/log"
	"net"
	"os"
	"path/filepath"
)

func ipcListen(endpoint string) (net.Listener, error) {
	if len(endpoint) > int(max_path_size) {
		log.Warn(fmt.Sprintf("The ipc endpoint is longer than %d characters. ", max_path_size),
			"endpoint", endpoint)
	}

	if err := os.MkdirAll(filepath.Dir(endpoint), 0751); err != nil {
		return nil, err
	}
	os.Remove(endpoint)
	l, err := net.Listen("unix", endpoint)
	if err != nil {
		return nil, err
	}
	os.Chmod(endpoint, 0600)
	return l, nil
}

func newIPCConnection(ctx context.Context, endpoint string) (net.Conn, error) {
	return new(net.Dialer).DialContext(ctx, "unix", endpoint)
}

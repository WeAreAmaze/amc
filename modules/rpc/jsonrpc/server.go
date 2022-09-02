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

package jsonrpc

import (
	"context"
	"github.com/amazechain/amc/log"
	mapset "github.com/deckarep/golang-set"
	"io"
	"sync/atomic"
)

const JSONRPCApi = "rpc"

type CodecOption int

type Server struct {
	services serviceRegistry
	run      int32
	codecs   mapset.Set
}

func NewServer() *Server {
	server := &Server{codecs: mapset.NewSet(), run: 1}
	rpcService := &RPCService{server}
	server.RegisterName(JSONRPCApi, rpcService)
	return server
}

func (s *Server) RegisterName(name string, receiver interface{}) error {
	return s.services.registerName(name, receiver)
}

func (s *Server) ServeCodec(codec ServerCodec, options CodecOption) {
	defer codec.close()

	if atomic.LoadInt32(&s.run) == 0 {
		return
	}

	s.codecs.Add(codec)
	defer s.codecs.Remove(codec)

	c := initClient(codec, &s.services)
	<-codec.closed()
	c.Close()
}

func (s *Server) serveSingleRequest(ctx context.Context, codec ServerCodec) {
	if atomic.LoadInt32(&s.run) == 0 {
		return
	}

	h := newHandler(ctx, codec, &s.services)
	h.allowSubscribe = false
	defer h.close(io.EOF, nil)

	req, err := codec.readBatch()
	if err != nil {
		if err != io.EOF {
			codec.writeJSON(ctx, errorMessage(&invalidMessageError{"parse error"}))
		}
		return
	}
	h.handleMsg(req)
}

func (s *Server) Stop() {
	if atomic.CompareAndSwapInt32(&s.run, 1, 0) {
		log.Debug("RPC server shutting down")
		s.codecs.Each(func(c interface{}) bool {
			c.(ServerCodec).close()
			return true
		})
	}
}

type RPCService struct {
	server *Server
}

func (s *RPCService) Modules() map[string]string {
	s.server.services.mu.Lock()
	defer s.server.services.mu.Unlock()

	modules := make(map[string]string)
	for name := range s.server.services.services {
		modules[name] = "1.0"
	}
	return modules
}

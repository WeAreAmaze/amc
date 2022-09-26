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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/amazechain/amc/log"
	"net/url"
	"reflect"
	"strconv"
	"sync/atomic"
	"time"
)

var (
	ErrClientQuit                = errors.New("client is closed")
	ErrNoResult                  = errors.New("no result in JSON-RPC response")
	ErrSubscriptionQueueOverflow = errors.New("subscription queue overflow")
	errClientReconnected         = errors.New("client reconnected")
	errDead                      = errors.New("connection lost")
)

const (
	defaultDialTimeout = 10 * time.Second
	subscribeTimeout   = 5 * time.Second
)

const (
	// Subscriptions are removed when the subscriber cannot keep up.
	//
	// This can be worked around by supplying a channel with sufficiently sized buffer,
	// but this can be inconvenient and hard to explain in the docs. Another issue with
	// buffered channels is that the buffer is static even though it might not be needed
	// most of the time.
	//
	// The approach taken here is to maintain a per-subscription linked list buffer
	// shrinks on demand. If the buffer reaches the size below, the subscription is
	// dropped.
	maxClientSubscriptionBuffer = 20000
)

type Client struct {
	idgen    func() ID // for subscriptions
	isHTTP   bool
	services *serviceRegistry

	idCounter     uint32
	reconnectFunc reconnectFunc

	writeConn jsonWriter

	close       chan struct{}
	closing     chan struct{}
	didClose    chan struct{}
	reconnected chan ServerCodec
	readOp      chan readOp
	readErr     chan error
	reqInit     chan *requestOp
	reqSent     chan error
	reqTimeout  chan *requestOp
}

type reconnectFunc func(ctx context.Context) (ServerCodec, error)

type clientContextKey struct{}

type clientConn struct {
	codec   ServerCodec
	handler *handler
}

func (c *Client) newClientConn(conn ServerCodec) *clientConn {
	ctx := context.WithValue(context.Background(), clientContextKey{}, c)
	handler := newHandler(ctx, conn, c.idgen, c.services)
	return &clientConn{conn, handler}
}

func (cc *clientConn) close(err error, inflightReq *requestOp) {
	cc.handler.close(err, inflightReq)
	cc.codec.close()
}

type readOp struct {
	msgs  []*jsonrpcMessage
	batch bool
}

type requestOp struct {
	ids  []json.RawMessage
	err  error
	resp chan *jsonrpcMessage
}

func (op *requestOp) wait(ctx context.Context, c *Client) (*jsonrpcMessage, error) {
	select {
	case <-ctx.Done():
		if !c.isHTTP {
			select {
			case c.reqTimeout <- op:
			case <-c.closing:
			}
		}
		return nil, ctx.Err()
	case resp := <-op.resp:
		return resp, op.err
	}
}

func Dial(rawurl string) (*Client, error) {
	return DialContext(context.Background(), rawurl)
}

func DialContext(ctx context.Context, rawurl string) (*Client, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "http", "https":
		return DialHTTP(rawurl)
	case "":
		return DialIPC(ctx, rawurl)
	default:
		return nil, fmt.Errorf("no known transport for URL scheme %q", u.Scheme)
	}
}
func ClientFromContext(ctx context.Context) (*Client, bool) {
	client, ok := ctx.Value(clientContextKey{}).(*Client)
	return client, ok
}

func newClient(initctx context.Context, connect reconnectFunc) (*Client, error) {
	conn, err := connect(initctx)
	if err != nil {
		return nil, err
	}
	c := initClient(conn, randomIDGenerator(), new(serviceRegistry))
	c.reconnectFunc = connect
	return c, nil
}

func initClient(conn ServerCodec, idgen func() ID, services *serviceRegistry) *Client {
	_, isHTTP := conn.(*httpConn)
	c := &Client{
		isHTTP:      isHTTP,
		idgen:       idgen,
		services:    services,
		writeConn:   conn,
		close:       make(chan struct{}),
		closing:     make(chan struct{}),
		didClose:    make(chan struct{}),
		reconnected: make(chan ServerCodec),
		readOp:      make(chan readOp),
		readErr:     make(chan error),
		reqInit:     make(chan *requestOp),
		reqSent:     make(chan error, 1),
		reqTimeout:  make(chan *requestOp),
	}
	if !isHTTP {
		go c.dispatch(conn)
	}
	return c
}

func (c *Client) RegisterName(name string, receiver interface{}) error {
	return c.services.registerName(name, receiver)
}

func (c *Client) nextID() json.RawMessage {
	id := atomic.AddUint32(&c.idCounter, 1)
	return strconv.AppendUint(nil, uint64(id), 10)
}

func (c *Client) SupportedModules() (map[string]string, error) {
	var result map[string]string
	ctx, cancel := context.WithTimeout(context.Background(), subscribeTimeout)
	defer cancel()
	err := c.CallContext(ctx, &result, "rpc_modules")
	return result, err
}

func (c *Client) Close() {
	if c.isHTTP {
		return
	}
	select {
	case c.close <- struct{}{}:
		<-c.didClose
	case <-c.didClose:
	}
}

func (c *Client) SetHeader(key, value string) {
	if !c.isHTTP {
		return
	}
	conn := c.writeConn.(*httpConn)
	conn.mu.Lock()
	conn.headers.Set(key, value)
	conn.mu.Unlock()
}

func (c *Client) Call(result interface{}, method string, args ...interface{}) error {
	ctx := context.Background()
	return c.CallContext(ctx, result, method, args...)
}

func (c *Client) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	if result != nil && reflect.TypeOf(result).Kind() != reflect.Ptr {
		return fmt.Errorf("call result parameter must be pointer or nil interface: %v", result)
	}
	msg, err := c.newMessage(method, args...)
	if err != nil {
		return err
	}
	op := &requestOp{ids: []json.RawMessage{msg.ID}, resp: make(chan *jsonrpcMessage, 1)}

	if c.isHTTP {
		err = c.sendHTTP(ctx, op, msg)
	} else {
		err = c.send(ctx, op, msg)
	}
	if err != nil {
		return err
	}

	switch resp, err := op.wait(ctx, c); {
	case err != nil:
		return err
	case resp.Error != nil:
		return resp.Error
	case len(resp.Result) == 0:
		return ErrNoResult
	default:
		return json.Unmarshal(resp.Result, &result)
	}
}

func (c *Client) Notify(ctx context.Context, method string, args ...interface{}) error {
	op := new(requestOp)
	msg, err := c.newMessage(method, args...)
	if err != nil {
		return err
	}
	msg.ID = nil

	if c.isHTTP {
		return c.sendHTTP(ctx, op, msg)
	}
	return c.send(ctx, op, msg)
}

func (c *Client) newMessage(method string, paramsIn ...interface{}) (*jsonrpcMessage, error) {
	msg := &jsonrpcMessage{Version: vsn, ID: c.nextID(), Method: method}
	if paramsIn != nil { // prevent sending "params":null
		var err error
		if msg.Params, err = json.Marshal(paramsIn); err != nil {
			return nil, err
		}
	}
	return msg, nil
}

func (c *Client) send(ctx context.Context, op *requestOp, msg interface{}) error {
	select {
	case c.reqInit <- op:
		err := c.write(ctx, msg, false)
		c.reqSent <- err
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closing:
		return ErrClientQuit
	}
}

func (c *Client) write(ctx context.Context, msg interface{}, retry bool) error {
	if c.writeConn == nil {
		if err := c.reconnect(ctx); err != nil {
			return err
		}
	}
	err := c.writeConn.writeJSON(ctx, msg)
	if err != nil {
		c.writeConn = nil
		if !retry {
			return c.write(ctx, msg, true)
		}
	}
	return err
}

func (c *Client) reconnect(ctx context.Context) error {
	if c.reconnectFunc == nil {
		return errDead
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, defaultDialTimeout)
		defer cancel()
	}
	newconn, err := c.reconnectFunc(ctx)
	if err != nil {
		log.Debug("RPC client reconnect failed", "err", err)
		return err
	}
	select {
	case c.reconnected <- newconn:
		c.writeConn = newconn
		return nil
	case <-c.didClose:
		newconn.close()
		return ErrClientQuit
	}
}

func (c *Client) dispatch(codec ServerCodec) {
	var (
		lastOp      *requestOp
		reqInitLock = c.reqInit
		conn        = c.newClientConn(codec)
		reading     = true
	)
	defer func() {
		close(c.closing)
		if reading {
			conn.close(ErrClientQuit, nil)
			c.drainRead()
		}
		close(c.didClose)
	}()

	go c.read(codec)

	for {
		select {
		case <-c.close:
			return

		case op := <-c.readOp:
			if op.batch {
				conn.handler.handleBatch(op.msgs)
			} else {
				conn.handler.handleMsg(op.msgs[0])
			}

		case err := <-c.readErr:
			log.Debug("RPC connection read error", "err", err)
			conn.close(err, lastOp)
			reading = false

		case newcodec := <-c.reconnected:
			log.Debug("RPC client reconnected", "reading", reading, "conn", newcodec.remoteAddr())
			if reading {
				conn.close(errClientReconnected, lastOp)
				c.drainRead()
			}
			go c.read(newcodec)
			reading = true
			conn = c.newClientConn(newcodec)
			conn.handler.addRequestOp(lastOp)

		case op := <-reqInitLock:
			reqInitLock = nil
			lastOp = op
			conn.handler.addRequestOp(op)

		case err := <-c.reqSent:
			if err != nil {
				conn.handler.removeRequestOp(lastOp)
			}
			reqInitLock = c.reqInit
			lastOp = nil

		case op := <-c.reqTimeout:
			conn.handler.removeRequestOp(op)
		}
	}
}

func (c *Client) drainRead() {
	for {
		select {
		case <-c.readOp:
		case <-c.readErr:
			return
		}
	}
}

func (c *Client) read(codec ServerCodec) {
	for {
		msgs, batch, err := codec.readBatch()
		if _, ok := err.(*json.SyntaxError); ok {
			codec.writeJSON(context.Background(), errorMessage(&parseError{err.Error()}))
		}
		if err != nil {
			c.readErr <- err
			return
		}
		c.readOp <- readOp{msgs, batch}
	}
}

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
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/amazechain/amc/log"
)

var (
	ErrNotificationsUnsupported = errors.New("notifications not supported")
	ErrSubscriptionNotFound     = errors.New("subscription not found")
)

type handler struct {
	reg            *serviceRegistry
	respWait       map[string]*requestOp // active client requests
	callWG         sync.WaitGroup        // pending call goroutines
	rootCtx        context.Context       // canceled by close()
	cancelRoot     func()                // cancel function for rootCtx
	conn           jsonWriter            // where responses will be sent
	allowSubscribe bool
}

type callProc struct {
	ctx context.Context
}

func newHandler(connCtx context.Context, conn jsonWriter, reg *serviceRegistry) *handler {
	rootCtx, cancelRoot := context.WithCancel(connCtx)
	h := &handler{
		reg:        reg,
		conn:       conn,
		respWait:   make(map[string]*requestOp),
		rootCtx:    rootCtx,
		cancelRoot: cancelRoot,
	}
	if conn.remoteAddr() != "" {
	}
	return h
}

func (h *handler) handleMsg(msg *jsonrpcMessage) {
	if ok := h.handleImmediate(msg); ok {
		return
	}
	h.startCallProc(func(cp *callProc) {
		answer := h.handleCallMsg(cp, msg)
		if answer != nil {
			h.conn.writeJSON(cp.ctx, answer)
		}
	})
}

func (h *handler) close(err error, inflightReq *requestOp) {
	h.cancelAllRequests(err, inflightReq)
	h.callWG.Wait()
	h.cancelRoot()
}

func (h *handler) addRequestOp(op *requestOp) {
	for _, id := range op.ids {
		h.respWait[string(id)] = op
	}
}

func (h *handler) removeRequestOp(op *requestOp) {
	for _, id := range op.ids {
		delete(h.respWait, string(id))
	}
}

func (h *handler) cancelAllRequests(err error, inflightReq *requestOp) {
	didClose := make(map[*requestOp]bool)
	if inflightReq != nil {
		didClose[inflightReq] = true
	}

	for id, op := range h.respWait {
		// Remove the op so that later calls will not close op.resp again.
		delete(h.respWait, id)

		if !didClose[op] {
			op.err = err
			close(op.resp)
			didClose[op] = true
		}
	}
}

func (h *handler) startCallProc(fn func(*callProc)) {
	h.callWG.Add(1)
	go func() {
		ctx, cancel := context.WithCancel(h.rootCtx)
		defer h.callWG.Done()
		defer cancel()
		fn(&callProc{ctx: ctx})
	}()
}

func (h *handler) handleImmediate(msg *jsonrpcMessage) bool {
	start := time.Now()
	switch {
	case msg.isNotification():
		if strings.HasSuffix(msg.Method, notificationMethodSuffix) {
			h.handleSubscriptionResult(msg)
			return true
		}
		return false
	case msg.isResponse():
		h.handleResponse(msg)
		log.Debug("Handled RPC response", "reqid", idForLog{msg.ID}, "t", time.Since(start))
		return true
	default:
		return false
	}
}

func (h *handler) handleSubscriptionResult(msg *jsonrpcMessage) {
	var result subscriptionResult
	if err := json.Unmarshal(msg.Params, &result); err != nil {
		log.Debug("Dropping invalid subscription message")
		return
	}
}

func (h *handler) handleResponse(msg *jsonrpcMessage) {
	op := h.respWait[string(msg.ID)]
	if op == nil {
		log.Debug("Unsolicited RPC response", "reqid", idForLog{msg.ID})
		return
	}
	delete(h.respWait, string(msg.ID))
	op.resp <- msg
}

func (h *handler) handleCallMsg(ctx *callProc, msg *jsonrpcMessage) *jsonrpcMessage {
	start := time.Now()
	switch {
	case msg.isCall():
		resp := h.handleCall(ctx, msg)
		var ctx []interface{}
		ctx = append(ctx, "reqid", idForLog{msg.ID}, "t", time.Since(start), "p", string(msg.Params), "r", string(resp.Result))
		if resp.Error != nil {
			ctx = append(ctx, "err", resp.Error.Message)
			if resp.Error.Data != nil {
				ctx = append(ctx, "errdata", resp.Error.Data)
			}
			log.Warn("Served "+msg.Method, ctx)
		} else {
			log.Debugf("Served "+msg.Method, ctx)
		}
		return resp
	case msg.hasValidID():
		return msg.errorResponse(&invalidRequestError{"invalid request"})
	default:
		return errorMessage(&invalidRequestError{"invalid request"})
	}
}

func (h *handler) handleCall(cp *callProc, msg *jsonrpcMessage) *jsonrpcMessage {
	callb := h.reg.callback(msg.Method)
	if callb == nil {
		return msg.errorResponse(&methodNotFoundError{method: msg.Method})
	}
	args, err := parsePositionalArguments(msg.Params, callb.argTypes)
	if err != nil {
		return msg.errorResponse(&invalidParamsError{err.Error()})
	}
	answer := h.runMethod(cp.ctx, msg, callb, args)
	return answer
}

func (h *handler) runMethod(ctx context.Context, msg *jsonrpcMessage, callb *callback, args []reflect.Value) *jsonrpcMessage {
	result, err := callb.call(ctx, msg.Method, args)
	if err != nil {
		return msg.errorResponse(err)
	}
	return msg.response(result)
}

type idForLog struct{ json.RawMessage }

func (id idForLog) String() string {
	if s, err := strconv.Unquote(string(id.RawMessage)); err == nil {
		return s
	}
	return string(id.RawMessage)
}

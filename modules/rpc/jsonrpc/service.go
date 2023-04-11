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
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unicode"
)

var (
	contextType      = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType        = reflect.TypeOf((*error)(nil)).Elem()
	subscriptionType = reflect.TypeOf(Subscription{})
	stringType       = reflect.TypeOf("")
)

type serviceRegistry struct {
	mu       sync.Mutex
	services map[string]service
}

type service struct {
	name          string
	callbacks     map[string]*callback
	subscriptions map[string]*callback // available subscriptions/notifications
}

type callback struct {
	fn          reflect.Value
	rcvr        reflect.Value
	argTypes    []reflect.Type
	hasCtx      bool
	errPos      int
	isSubscribe bool // true if this is a subscription callback
}

func (r *serviceRegistry) registerName(name string, rcvr interface{}) error {
	rcvrVal := reflect.ValueOf(rcvr)
	if name == "" {
		return fmt.Errorf("no service name for type %s", rcvrVal.Type().String())
	}
	callbacks := suitableCallbacks(rcvrVal)
	if len(callbacks) == 0 {
		return fmt.Errorf("service %T doesn't have any suitable methods/subscriptions to expose", rcvr)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.services == nil {
		r.services = make(map[string]service)
	}
	svc, ok := r.services[name]
	if !ok {
		svc = service{
			name:          name,
			callbacks:     make(map[string]*callback),
			subscriptions: make(map[string]*callback),
		}
		r.services[name] = svc
	}
	for name, cb := range callbacks {
		if cb.isSubscribe {
			svc.subscriptions[name] = cb
		} else {
			svc.callbacks[name] = cb
		}
	}
	return nil
}

func (r *serviceRegistry) callback(method string) *callback {
	elem := strings.SplitN(method, serviceMethodSeparator, 2)
	if len(elem) != 2 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// todo
	//log.Info("meth %+v", r.services[elem[0]].callbacks)
	return r.services[elem[0]].callbacks[elem[1]]
}

// subscription returns a subscription callback in the given service.
func (r *serviceRegistry) subscription(service, name string) *callback {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.services[service].subscriptions[name]
}

func suitableCallbacks(receiver reflect.Value) map[string]*callback {
	typ := receiver.Type()
	callbacks := make(map[string]*callback)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		if method.PkgPath != "" {
			continue
		}
		cb := newCallback(receiver, method.Func)
		if cb == nil {
			continue
		}
		name := formatName(method.Name)
		callbacks[name] = cb
	}
	return callbacks
}

func newCallback(receiver, fn reflect.Value) *callback {
	fntype := fn.Type()
	c := &callback{fn: fn, rcvr: receiver, errPos: -1, isSubscribe: isPubSub(fntype)}
	c.makeArgTypes()

	outs := make([]reflect.Type, fntype.NumOut())
	for i := 0; i < fntype.NumOut(); i++ {
		outs[i] = fntype.Out(i)
	}
	if len(outs) > 2 {
		return nil
	}
	switch {
	case len(outs) == 1 && isErrorType(outs[0]):
		c.errPos = 0
	case len(outs) == 2:
		if isErrorType(outs[0]) || !isErrorType(outs[1]) {
			return nil
		}
		c.errPos = 1
	}
	return c
}

func (c *callback) makeArgTypes() {
	fntype := c.fn.Type()
	firstArg := 0
	if c.rcvr.IsValid() {
		firstArg++
	}
	if fntype.NumIn() > firstArg && fntype.In(firstArg) == contextType {
		c.hasCtx = true
		firstArg++
	}
	c.argTypes = make([]reflect.Type, fntype.NumIn()-firstArg)
	for i := firstArg; i < fntype.NumIn(); i++ {
		c.argTypes[i-firstArg] = fntype.In(i)
	}
}

func (c *callback) call(ctx context.Context, method string, args []reflect.Value) (res interface{}, errRes error) {
	fullargs := make([]reflect.Value, 0, 2+len(args))
	if c.rcvr.IsValid() {
		fullargs = append(fullargs, c.rcvr)
	}
	if c.hasCtx {
		fullargs = append(fullargs, reflect.ValueOf(ctx))
	}
	fullargs = append(fullargs, args...)

	defer func() {
		// if err := recover(); err != nil {
		// 	const size = 64 << 10
		// 	buf := make([]byte, size)
		// 	buf = buf[:runtime.Stack(buf, false)]
		// 	log.Error("RPC method " + method + " crashed: " + fmt.Sprintf("%v\n%s", err, buf))
		// 	errRes = errors.New("method handler crashed")
		// }
	}()
	results := c.fn.Call(fullargs)
	if len(results) == 0 {
		return nil, nil
	}
	if c.errPos >= 0 && !results[c.errPos].IsNil() {
		err := results[c.errPos].Interface().(error)
		return reflect.Value{}, err
	}
	return results[0].Interface(), nil
}

// Is t context.Context or *context.Context?
func isContextType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t == contextType
}

func isErrorType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Implements(errorType)
}

// Is t Subscription or *Subscription?
func isSubscriptionType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t == subscriptionType
}

// isPubSub tests whether the given method has as as first argument a context.Context and
// returns the pair (Subscription, error).
func isPubSub(methodType reflect.Type) bool {
	// numIn(0) is the receiver type
	if methodType.NumIn() < 2 || methodType.NumOut() != 2 {
		return false
	}
	return isContextType(methodType.In(1)) &&
		isSubscriptionType(methodType.Out(0)) &&
		isErrorType(methodType.Out(1))
}

func formatName(name string) string {
	ret := []rune(name)
	if len(ret) > 0 {
		ret[0] = unicode.ToLower(ret[0])
	}
	return string(ret)
}

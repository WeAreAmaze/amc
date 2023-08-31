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

package v2

import (
	"errors"
	"reflect"
	"sync"
)

var errBadChannel = errors.New("event: Subscribe argument does not have sendable channel type")

var GlobalEvent Event

type Subscription interface {
	Err() <-chan error // returns the error channel
	Unsubscribe()      // cancels sending of events, closing the error channel
}

type Event struct {
	once sync.Once

	mu     sync.Mutex
	inbox2 map[string]caseList
}

func (e *Event) init() {
	e.inbox2 = make(map[string]caseList)
}

func (e *Event) Subscribe(channel interface{}) Subscription {
	e.once.Do(e.init)

	chanval := reflect.ValueOf(channel)
	chantyp := chanval.Type()
	if chantyp.Kind() != reflect.Chan || chantyp.ChanDir()&reflect.SendDir == 0 {
		panic(errBadChannel)
	}
	sub := &eventSub{feed: e, channel: chanval, err: make(chan error, 1)}
	e.mu.Lock()
	defer e.mu.Unlock()

	cas := reflect.SelectCase{Dir: reflect.SelectSend, Chan: chanval}
	key := reflect.TypeOf(channel).Elem().String()

	//e.inbox2[key][0] is a channel used to interrupts Send
	if len(e.inbox2[key]) < 1 {
		e.inbox2[key] = append(e.inbox2[key], reflect.SelectCase{Chan: reflect.ValueOf(make(chan interface{})), Dir: reflect.SelectRecv})
	}
	e.inbox2[key] = append(e.inbox2[key], cas)

	return sub
}

func (e *Event) Send(value interface{}) int {

	rvalue := reflect.ValueOf(value)
	chantyp := reflect.TypeOf(value)
	e.once.Do(e.init)

	e.mu.Lock()
	defer e.mu.Unlock()
	if chantyp.Kind() != reflect.Ptr {
		panic("value must be ptr type!!")
	}

	key := chantyp.Elem().String()
	cases := e.inbox2[key]
	nsent := 0

	// Set the sent value on all channels.
	for i := 1; i < len(cases); i++ {
		cases[i].Send = rvalue
	}
	for {
		// Fast path: try sending without blocking before adding to the select set.
		// This should usually succeed if subscribers are fast enough and have free
		// buffer space.
		for i := 1; i < len(cases); i++ {
			if cases[i].Chan.TrySend(rvalue.Elem()) {
				nsent++
				cases = cases.deactivate(i)
				i--
			}
		}
		if len(cases) == 1 {
			break
		}
		// Select on the blocked receivers, waiting for them to unblock.
		chosen, recv, _ := reflect.Select(cases)
		if chosen == 0 /* <-f.removeSub */ {
			index := e.inbox2[key].find(recv.Interface())
			e.inbox2[key] = e.inbox2[key].delete(index)
			if index >= 0 && index < len(cases) {
				// Shrink 'cases' too because the removed case was still active.
				cases = e.inbox2[key][:len(cases)-1]
			}
		} else {
			cases = cases.deactivate(chosen)
			nsent++
		}
	}

	return nsent
}

func (e *Event) remove(sub *eventSub) {
	ch := sub.channel.Interface()
	key := reflect.TypeOf(ch).Elem().String()
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.inbox2[key]; ok {
		if index := e.inbox2[key].find(ch); index != -1 {
			e.inbox2[key] = e.inbox2[key].delete(index)
			if len(e.inbox2[key]) <= 0 {
				delete(e.inbox2, key)
			}
		}
	}
}

type eventSub struct {
	feed    *Event
	channel reflect.Value
	errOnce sync.Once
	err     chan error
}

func (sub *eventSub) Unsubscribe() {
	sub.errOnce.Do(func() {
		sub.feed.remove(sub)
		close(sub.err)
	})
}

func (sub *eventSub) Err() <-chan error {
	return sub.err
}

type caseList []reflect.SelectCase

func (cs caseList) find(channel interface{}) int {
	for i, cas := range cs {
		if cas.Chan.Interface() == channel {
			return i
		}
	}

	return -1
}

func (cs caseList) delete(index int) caseList {
	return append(cs[:index], cs[index+1:]...)
}

func (cs caseList) deactivate(index int) caseList {
	last := len(cs) - 1
	cs[index], cs[last] = cs[last], cs[index]
	return cs[:last]
}

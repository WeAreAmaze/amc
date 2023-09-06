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
	"github.com/amazechain/amc/log"
	"reflect"
	"sync"
)

var GlobalEvent Event

type Event struct {
	once sync.Once

	feeds      map[string]*Feed
	feedsLock  sync.RWMutex
	feedsScope map[string]*SubscriptionScope
}

func (e *Event) init() {
	e.feeds = make(map[string]*Feed)
	e.feedsScope = make(map[string]*SubscriptionScope)

}

func (e *Event) initKey(key string) {
	e.feedsLock.Lock()
	defer e.feedsLock.Unlock()
	if _, ok := e.feeds[key]; !ok {
		e.feeds[key] = new(Feed)
		e.feedsScope[key] = new(SubscriptionScope)
	}
}

func (e *Event) Subscribe(channel interface{}) Subscription {
	e.once.Do(e.init)

	key := reflect.TypeOf(channel).Elem().String()
	e.initKey(key)

	e.feedsLock.RLock()
	defer e.feedsLock.RUnlock()
	sub := e.feedsScope[key].Track(e.feeds[key].Subscribe(channel))

	return sub
}

func (e *Event) Send(value interface{}) int {

	e.once.Do(e.init)

	key := reflect.TypeOf(value).String()
	e.initKey(key)

	nsent := 0

	e.feedsLock.RLock()
	defer e.feedsLock.RUnlock()

	log.Trace("GlobalEvent Send", "key", key)
	if e.feedsScope[key].Count() == 0 {
		return nsent
	}
	nsent = e.feeds[key].Send(value)

	return nsent
}

func (e *Event) Close() {
	e.feedsLock.Lock()
	defer e.feedsLock.Unlock()

	for _, scope := range e.feedsScope {
		scope.Close()
	}
}

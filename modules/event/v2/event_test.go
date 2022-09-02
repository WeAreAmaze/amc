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
	"fmt"
	"reflect"
	"sync"
	"testing"
)

type A struct {
	A string
}

func TestEvent_Subscribe(t *testing.T) {
	var feed Event

	c1 := make(chan *int, 10)
	c2 := make(chan *A, 10)

	feed.Subscribe(c1)
	feed.Subscribe(c2)
}

func TestEvent_Send2(t *testing.T) {
	c := make(chan int, 1)
	vc := reflect.ValueOf(c)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		a := <-c
		t.Log("a value ", a)
	}()

	// use of TrySend() method
	succeeded := vc.TrySend(reflect.ValueOf(123))

	fmt.Println(succeeded, vc.Len(), vc.Cap())
	wg.Wait()
}

func TestEvent_Send(t *testing.T) {
	var feed Event
	ch1 := make(chan int, 1)
	ch2 := make(chan A, 1)

	sub1 := feed.Subscribe(ch1)
	sub2 := feed.Subscribe(ch2)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		a := <-ch1
		t.Log("ch1 Value", a)
	}()

	go func() {
		defer wg.Done()
		a := <-ch2
		t.Log("ch2 Value", a)
	}()
	a := 2
	feed.Send(&a)
	feed.Send(&A{
		A: "123",
	})
	wg.Wait()

	sub1.Unsubscribe()
	sub2.Unsubscribe()
}

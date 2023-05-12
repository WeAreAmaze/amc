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

package event

import (
	"fmt"
	"github.com/amazechain/amc/api/protocol/msg_proto"
	"reflect"
	"strconv"
	"testing"
	"time"
)

//func TestEvent_Send(t *testing.T) {
//	var (
//		event Event
//		count = 2000
//
//		ch1 = make(chan int, 5)
//		ch2 = make(chan uint64, 5)
//
//		wg sync.WaitGroup
//	)
//
//	sub1 := event.Subscribe(ch1)
//	defer sub1.Unsubscribe()
//	sub2 := event.Subscribe(ch2)
//	defer sub2.Unsubscribe()
//
//	wg.Add(2)
//	run := func(ch chan int, name string) {
//		defer wg.Done()
//		for {
//			select {
//			case n, ok := <-ch:
//				if !ok {
//					return
//				}
//				t.Logf("%s recv %d", name, n)
//			case err := <-sub2.Err():
//				t.Log(err)
//
//			}
//		}
//	}
//
//	run2 := func(ch chan uint64, name string) {
//		defer wg.Done()
//		for {
//			select {
//			case n, ok := <-ch:
//				if !ok {
//					return
//				}
//				t.Logf("%s recv %d", name, n)
//			case err := <-sub2.Err():
//				t.Log(err)
//
//			}
//		}
//	}
//
//	go run(ch1, "ch-1")
//	go run2(ch2, "ch-2")
//
//	for i := 0; i < count; i++ {
//		event.Send(i)
//		//time.Sleep(100 * time.Millisecond)
//	}
//}

func TestTime(t *testing.T) {
	var timestamp int64 = 1615594634
	tm := time.Unix(timestamp, 0)
	t.Log("time", tm.String())
}

func TestSelect(t *testing.T) {
	var chs1 = make(chan int)
	var chs2 = make(chan float64)
	var chs3 = make(chan string)
	var ch4close = make(chan int)
	defer close(ch4close)

	go func(c chan int, ch4close chan int) {
		for i := 0; i < 5; i++ {
			c <- i
		}
		close(c)
		ch4close <- 1
	}(chs1, ch4close)

	go func(c chan float64, ch4close chan int) {
		for i := 0; i < 5; i++ {
			c <- float64(i) + 0.1
		}
		close(c)
		ch4close <- 1
	}(chs2, ch4close)

	go func(c chan string, ch4close chan int) {
		for i := 0; i < 5; i++ {
			c <- "string:" + strconv.Itoa(i)
		}
		close(c)
		ch4close <- 1
	}(chs3, ch4close)

	var selectCase = make([]reflect.SelectCase, 4)
	selectCase[0].Dir = reflect.SelectRecv
	selectCase[0].Chan = reflect.ValueOf(chs1)

	selectCase[1].Dir = reflect.SelectRecv
	selectCase[1].Chan = reflect.ValueOf(chs2)

	selectCase[2].Dir = reflect.SelectRecv
	selectCase[2].Chan = reflect.ValueOf(chs3)

	selectCase[3].Dir = reflect.SelectRecv
	selectCase[3].Chan = reflect.ValueOf(ch4close)

	done := 0
	finished := 0
	for finished < len(selectCase)-1 {
		chosen, recv, recvOk := reflect.Select(selectCase)

		if recvOk {
			done = done + 1
			switch chosen {
			case 0:
				fmt.Println(chosen, recv.Int())
			case 1:
				fmt.Println(chosen, recv.Float())
			case 2:
				fmt.Println(chosen, recv.String())
			case 3:
				finished = finished + 1
				done = done - 1
				// fmt.Println("finished\t", finished)
			}
		}
	}
	fmt.Println("Done", done)
}

type AB struct {
	A string
	B string
}

func TestReflect(t *testing.T) {
	num := uint64(22)
	s := msg_proto.MessageData{
		ClientVersion: "1",
		Timestamp:     0,
		Id:            "1",
		NodeID:        "2",
		NodePubKey:    nil,
		Sign:          nil,
		Payload:       nil,
		Gossip:        false,
	}

	//sCh := make(chan solo.SoloMiner, 2)

	t1 := reflect.TypeOf(num)
	t2 := reflect.TypeOf(s)
	//t3 := reflect.TypeOf(sCh)

	t.Logf("t1 type: %s", t1.String())
	t.Logf("t2 type: %s", t2.String())
	//t.Logf("t3 type: %s", t3.Elem().String())
}

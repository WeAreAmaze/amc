// Copyright 2023 The AmazeChain Authors
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

package jsonrpc_test

import (
	"context"
	"fmt"
	"github.com/amazechain/amc/common/hexutil"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
)

// In this example, our client wishes to track the latest 'block number'
// known to the server. The server supports two methods:
//
// eth_getBlockByNumber("latest", {})
//    returns the latest block object.
//
// eth_subscribe("newHeads")
//    creates a subscription which fires block objects when new blocks arrive.

type Block struct {
	Number *hexutil.Big
}

//func TestExampleClientSubscription(t *testing.T) {
//	// Connect the client.
//	client, err := jsonrpc.Dial("ws://127.0.0.1:20013")
//	if nil != err {
//		panic(err)
//	}
//	subch := make(chan Block)
//
//	// Ensure that subch receives the latest block.
//	//go func() {
//	//	for i := 0; ; i++ {
//	//		if i > 0 {
//	//			time.Sleep(2 * time.Second)
//	//		}
//	subscribeBlocks(client, subch)
//	//	}
//	//}()
//
//	// Print events from the subscription as they arrive.
//	for block := range subch {
//		fmt.Println("latest block:", block.Number)
//	}
//}

// subscribeBlocks runs in its own goroutine and maintains
// a subscription for new blocks.
func subscribeBlocks(client *jsonrpc.Client, subch chan Block) {
	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()

	ctx := context.Background()

	// Subscribe to new blocks.
	sub, err := client.Subscribe(ctx, "eth", subch, "newHeads")
	if err != nil {
		fmt.Println("subscribe error:", err)
		return
	}

	// The connection is established now.
	// Update the channel with the current block.
	//var lastBlock Block
	//err = client.CallContext(ctx, &lastBlock, "eth_getBlockByNumber", "latest", false)
	//if err != nil {
	//	fmt.Println("can't get latest block:", err)
	//	return
	//}
	//subch <- lastBlock

	// The subscription will deliver events to the channel. Wait for the
	// subscription to end for any reason, then loop around to re-establish
	// the connection.
	fmt.Println("connection lost: ", <-sub.Err())
}

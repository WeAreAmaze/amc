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

package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/crypto/bls"
	"github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules/state"
	"github.com/go-kit/kit/transport/http/jsonrpc"
	"github.com/gorilla/websocket"
	"os"
	"os/signal"
	"syscall"
)

var privateKey bls.SecretKey
var addressKey types.Address

func RootContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()

		ch := make(chan os.Signal, 1)
		defer close(ch)

		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)

		select {
		case sig := <-ch:
			log.Info("Got interrupt, shutting down...", "sig", sig)
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func main() {
	var err error
	sByte, err := hex.DecodeString("2d09d9f4e166f35a4ab0a2edd599e2a23bbe86b312b2e05b34d9fbe5693b1e48")
	if nil != err {
		panic(err)
	}

	var sb [32]byte
	copy(sb[:], sByte)
	privateKey, err = bls.SecretKeyFromRandom32Byte(sb)
	if nil != err {
		panic(err)
	}

	ecdPk, err := crypto.HexToECDSA("2d09d9f4e166f35a4ab0a2edd599e2a23bbe86b312b2e05b34d9fbe5693b1e48")
	if nil != err {
		panic(err)
	}
	addressKey = crypto.PubkeyToAddress(ecdPk.PublicKey)

	ctx, cancle := RootContext()
	defer cancle()

	con, _, err := websocket.DefaultDialer.DialContext(ctx, "ws://127.0.0.1:20013", nil)
	defer con.Close()

	end := make(chan struct{})
	defer close(end)

	go func() {
		for {
			select {
			case <-ctx.Done():
				end <- struct{}{}
				return
			default:
				typ, msg, err := con.ReadMessage()
				if nil != err {
					log.Errorf("read msg failed: %v", err)
					continue
				}
				if typ == websocket.TextMessage {
					fmt.Println("read msg: ", string(msg))
					params, err := unwrapJSONRPC(msg)
					if nil != err {
						log.Warn(err.Error())
						continue
					}

					bean := new(state.EntireCode)
					if err := json.Unmarshal(params, bean); err != nil {
						log.Errorf("unmarshal entire failed, %v", err)
						continue
					}

					root := verify(ctx, bean)
					res := aggsign.AggSign{}
					res.Number = bean.Entire.Header.Number.Uint64()
					res.Address = addressKey
					res.StateRoot = root
					copy(res.Sign[:], privateKey.Sign(root[:]).Marshal())
					in, err := json.Marshal(res)
					if nil != err {
						panic(err)
					}

					wrapRequest, _ := wrapJSONRPCRequest(in)
					if err := con.WriteMessage(websocket.TextMessage, wrapRequest); nil != err {
						log.Error("write msg failed: ", err)
					}
					log.Infof("write msg: %s", wrapRequest)
				}
			}
		}
	}()

	if err = con.PingHandler()(""); nil != err {
		panic(err)
	}

	if err := con.WriteMessage(websocket.TextMessage, []byte(`{
		"jsonrpc": "2.0",
		"method": "eth_subscribe",
		"params": [
		  "minedBlock",
		  "`+addressKey.String()+`"
		],
		"id": 1
	  }`)); err != nil {

		cancle()
	}

	<-end
}

func unwrapJSONRPC(in []byte) ([]byte, error) {
	//"{\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32000,\"message\":\"unauthed address: 0xeB156a42dcaFcf155B07f3638892440C7dE5d564\"}}\n"
	//ws consumer received msg:%!(EXTRA string=ws consumer received msg:, string={"jsonrpc":"2.0","id":1,"result":"0x96410b68a9f8875bb20fde06823eb861"}
	req := new(jsonrpc.Request)
	if err := json.Unmarshal(in, req); err != nil {
		return nil, err
	}
	if len(req.Params) == 0 {
		return []byte{}, errors.New("empty request params")
	}

	//type innerProtocolEntire struct {
	//	Entire json.RawMessage `json:"Entire"`
	//}
	type innerProtocol struct {
		Subscription string          `json:"subscription"`
		Result       json.RawMessage `json:"result"`
	}

	innerReq := new(innerProtocol)
	if err := json.Unmarshal(req.Params, innerReq); err != nil {
		return nil, err
	}

	return innerReq.Result, nil
}

type JSONRPCRequest struct {
	JsonRpc string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	ID      int               `json:"id"`
	Params  []json.RawMessage `json:"params"`
}

func wrapJSONRPCRequest(in []byte) ([]byte, error) {
	d := &JSONRPCRequest{
		JsonRpc: "2.0",
		Method:  "eth_submitSign",
		ID:      1,
		Params:  make([]json.RawMessage, 1),
	}
	d.Params[0] = in
	return json.Marshal(d)
}

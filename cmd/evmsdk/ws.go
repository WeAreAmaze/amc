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

package evmsdk

import (
	"encoding/json"
	"io/ioutil"
	"runtime"

	"github.com/gorilla/websocket"
)

type WebSocketService struct {
	addr      string
	acc       string
	readConn  *websocket.Conn
	writeConn *websocket.Conn
}

func NewWebSocketService(addr, acc string) (*WebSocketService, error) {
	simpleLog("init ws read conn")
	readConn, readResp, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		simpleLog("init ws read conn error,err=%+v", err)
		return nil, err
	}
	if readResp != nil {
		readRespBytes, err := ioutil.ReadAll(readResp.Body)
		if err != nil {
			simpleLog("readresp error", "err", err)
		}
		simpleLog("readresp", "readRespBytes", string(readRespBytes))
	}
	readConn.SetPongHandler(func(appData string) error { simpleLog("read pong", appData); return nil })
	readConn.SetPingHandler(func(appData string) error { simpleLog("read ping", appData); return nil })
	simpleLog("init ws read conn")

	writeConn, writeResp, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		simpleLog("init ws write conn error,err=%+v", err)
		return nil, err
	}
	if writeResp != nil {
		readRespBytes, err := ioutil.ReadAll(writeResp.Body)
		if err != nil {
			simpleLog("writeresp error", "err", err)
		}
		simpleLog("writeresp", "writeRespBytes", string(readRespBytes))
	}
	writeConn.SetPongHandler(func(appData string) error { simpleLog("write pong", appData); return nil })
	writeConn.SetPingHandler(func(appData string) error { simpleLog("write ping", appData); return nil })
	simpleLog("init dial ws done")
	return &WebSocketService{
		addr:      addr,
		readConn:  readConn,
		writeConn: writeConn,
		acc:       acc,
	}, nil
}

func (ws *WebSocketService) Chans(pubk string) (<-chan []byte, chan<- []byte, error) {
	simpleLog("ping read conn")
	err := ws.readConn.PingHandler()("")
	if err != nil {
		simpleLog("ping read conn error,err=%+v", err)
		return nil, nil, err
	}
	simpleLog("ping done")
	msg := `{
	"jsonrpc": "2.0",
	"method": "eth_subscribe",
	"params": [
	"minedBlock",
	"` + ws.acc + `"
	],
	"id": 1
}`
	simpleLog("minedBlock message:%+v", msg)

	if err := ws.readConn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		simpleLog("minedBlock message write message error,err=%+v", err)
		return nil, nil, err
	}

	_, msg0, err := ws.readConn.ReadMessage()
	if err != nil {
		simpleLog("init read conn readmessage error,err=", err)
		return nil, nil, err
	}
	msg0bean := new(JSONRPCRequest)
	if err := json.Unmarshal(msg0, msg0bean); err != nil {
		simpleLog("unmarshal message0 error,err=", err)
		return nil, nil, err
	}
	if msg0bean.Error != nil {
		if msgCode := msg0bean.Error["code"]; msgCode != nil {
			if msgCodeFloat, ok := msgCode.(float64); ok && msgCodeFloat != 0 {
				simpleLogf("init read conn error,code=%+v ,message=%+v", msgCodeFloat, msg0bean.Error["message"])
				return nil, nil, &InnerError{Code: int(msgCodeFloat), Msg: msg0bean.Error["message"].(string)}
			}
		}
	}

	chO, chI := make(chan []byte, 0), make(chan []byte, 0)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				buf := make([]byte, 4096)
				runtime.Stack(buf, true)
				simpleLog("readconn goroutine down", "err", err)
				simpleLog(string(buf))
			}
			EE.Stop()
		}()
		for {
			msgType, b, err := ws.readConn.ReadMessage()
			if err != nil {
				simpleLog("ws closed:", err.Error())
				close(chO)
				return
			}
			if msgType == websocket.TextMessage {
				chO <- b
			}
			simpleLog("received msg:", string(b))
		}
	}()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				buf := make([]byte, 4096)
				runtime.Stack(buf, true)
				simpleLog("writeconn goroutine down", "err", err)
				simpleLog(string(buf))
			}
			EE.Stop()
		}()
		for msg := range chI {
			/*
				{
					"jsonrpc":"2.0",
					"method":"eth_submitSign",
					"params":[{
						"number":11111,
						"stateRoot":"0x0000000000000000000000000000000000000000000000000000000000000000",
						"sign":"0xb4dd42744e40aa50bc0e747a52dcf8d1196c08a0f9fcfd8fda0aaa413a47bbdda20d1a76298642ee68b1f1009df149680e4e4b14f8c488faa4010ad9f26a81379c93d7f0da5596192da203c31a6301cd50d53734e537d828fe7f9593eb7035d3",
						"publicKey":"0xa8a236acd9b7ea68c1858a3059e4aad7698842f7de49ace8dc49acc922bd6d543792e98b7b74fa380bf622671792d57a"
					}],
					"id":1
				}
			*/
			wrappedRequest, err := ws.unwrapJSONRPCRequest(msg)
			if err != nil {
				simpleLog("wrapJSONRPCRequest error,err=", err)
				continue
			}
			if err := ws.writeConn.WriteMessage(websocket.TextMessage, wrappedRequest); err != nil {
				simpleLog("ws producer stopped")
				return
			}
			simpleLog("submit Sign to chain")
		}
	}()
	return chO, chI, nil
}

type JSONRPCRequest struct {
	JsonRpc string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	ID      int                    `json:"id"`
	Params  []json.RawMessage      `json:"params"`
	Error   map[string]interface{} `json:"error"`
}

func (ws *WebSocketService) unwrapJSONRPCRequest(in []byte) ([]byte, error) {
	d := &JSONRPCRequest{
		JsonRpc: "2.0",
		Method:  "eth_submitSign",
		ID:      1,
		Params:  make([]json.RawMessage, 1),
	}
	d.Params[0] = in
	return json.Marshal(d)

}

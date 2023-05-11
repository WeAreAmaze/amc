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
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/amazechain/amc/common/crypto"
	"reflect"
	"sync"
	"testing"
)

func TestGetNetInfos(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetNetInfos(); got != tt.want {
				t.Errorf("GetNetInfos() = %v, want %v", got, tt.want)
			} else {
				t.Log(got)
				fmt.Println(got)
			}
		})
	}
}

func TestEmit(t *testing.T) {
	type args struct {
		jsonText string
	}

	//settttting
	aa := Emit(`{"type":"test","val":{"app_base_path":"/Users/xan/misc"}}`)
	fmt.Println(aa)

	// tests := []struct {
	// 	name string
	// 	args args
	// }{{
	// 	name: "t1",
	// 	args: args{
	// 		jsonText: `{"type":"start"}`,
	// 	},
	// }, {
	// 	name: "t2",
	// 	args: args{
	// 		jsonText: `{"type":"test"}`,
	// 	},
	// },
	// // 	{
	// // 		name: "t3",
	// // 		args: args{
	// // 			jsonText: `{"type":"list"}`,
	// // 		},
	// // 	},
	// }
	// for _, tt := range tests {
	// 	t.Run(tt.name, func(t *testing.T) {
	// 		got := Emit(tt.args.jsonText)
	// 		if len(got) == 0 {
	// 			t.Errorf("Emit() = %v", got)
	// 		}
	// 		t.Log(got)
	// 		fmt.Println(got)
	// 	})
	// }

	// <-time.After(60 * time.Second)
}

func TestBlsSign(t *testing.T) {
	pk := make([]byte, 32)
	_, err := rand.Read(pk)
	if err != nil {
		t.Error(err)
	}

	req := `
{
	"type":"blssign",
	"val":{
		"priv_key":"` + hex.EncodeToString(pk) + `",
		"msg":"123123"
	}
}
	`
	resp := Emit(req)
	_ = resp
	fmt.Println(resp)
}

func TestEvmEngine_Start(t *testing.T) {
	type fields struct {
		mu          sync.Mutex
		ctx         context.Context
		cancelFunc  context.CancelFunc
		Account     string
		AppBasePath string
		State       string
		BlockChan   chan string
		PrivKey     string
		ServerUri   string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		struct {
			name    string
			fields  fields
			wantErr bool
		}{
			name: "t1",
			fields: fields{
				PrivKey:   `b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAaAAAABNlY2RzYS1zaGEyLW5pc3RwMjU2AAAACG5pc3RwMjU2AAAAQQSgodcFy6hKA9enyiAzEKfpg7Rib5AFx3w33V0NsMYOAoUXtbQtFIOIIgNsTBj9ei1DdGJ4QSbCgw3w37X7oXvkAAAAqAK68x8CuvMfAAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBKCh1wXLqEoD16fKIDMQp+mDtGJvkAXHfDfdXQ2wxg4ChRe1tC0Ug4giA2xMGP16LUN0YnhBJsKDDfDftfuhe+QAAAAhAMhPG7U/g7k9+YWm66Yk1yDhUvwkHmLMWvV/bTtGeElWAAAACW1hY0Bib2dvbgECAwQFBg==`,
				ServerUri: "ws://127.0.0.1:20013",
				Account:   "0x01",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EvmEngine{
				mu:          tt.fields.mu,
				ctx:         tt.fields.ctx,
				cancelFunc:  tt.fields.cancelFunc,
				Account:     tt.fields.Account,
				AppBasePath: tt.fields.AppBasePath,
				EngineState: tt.fields.State,
				BlockChan:   tt.fields.BlockChan,
				PrivKey:     tt.fields.PrivKey,
				ServerUri:   tt.fields.ServerUri,
			}
			if err := e.Start(); (err != nil) != tt.wantErr {
				t.Errorf("EvmEngine.Start() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEvmEngine_vertify(t *testing.T) {
	type fields struct {
		mu          sync.Mutex
		ctx         context.Context
		cancelFunc  context.CancelFunc
		Account     string
		AppBasePath string
		State       string
		BlockChan   chan string
		PrivKey     string
		ServerUri   string
	}
	type args struct {
		in []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "t1",
			fields: fields{
				PrivKey:   `37d15846af852c47649005fd6dfe33483d394aaa60c14e7c56f4deadb116329e`,
				ServerUri: "ws://127.0.0.1:20013",
				Account:   "0x01",
			},
			args: args{
				in: []byte(`
			{
				"entire":
				{
					"header":{
						"parentHash":"0xeab8f2dca7682e72c594bfe9da4d83fb623741b21c729f57a7ff06a69aa3be38",
						"sha3Uncles":"0xeab8f2dca7682e72c594bfe9da4d83fb623741b21c729f57a7ff06a69aa3be38",
						"miner":"0x588639773Bc6F163aa262245CDa746c120676431",
						"stateRoot":"0xf9e3e616232713a12a90a2abc2302d5fd2b6360764c204ebe2acda342390841b",
						"transactionsRoot":"0xf9e3e616232713a12a90a2abc2302d5fd2b6360764c204ebe2acda342390841b",
						"receiptsRoot":"0xf9e3e616232713a12a90a2abc2302d5fd2b6360764c204ebe2acda342390841b",
						"logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
						"difficulty":"0x2a65b3",
						"number":"0x1532d9",
						"gasLimit":"0x1c9c380",
						"gasUsed": "0x0",
						"timestamp":"0x638f0e18",
						"extraData": "0xd883010a17846765746888676f312e31382e35856c696e757800000000000000ab9c084cefea3b41830c827c91b8908b12f64b08a243ee4fabd28bdd4556154f00d9a08f7c4a6c2794a63100d92910595e6747db4af69d576c224373c707db4400"
					},
					"uncles":[],
					"transactions":[],
					"pprof":"0xf9e3e616232713a12a90a2abc2302d5fd2b6360764c204ebe2acda342390841b",
					"senders":[]
				},
				"codes":[]
			}`),
			},
			want:    []byte("321123bb"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EvmEngine{
				mu:          tt.fields.mu,
				ctx:         tt.fields.ctx,
				cancelFunc:  tt.fields.cancelFunc,
				Account:     tt.fields.Account,
				AppBasePath: tt.fields.AppBasePath,
				EngineState: tt.fields.State,
				BlockChan:   tt.fields.BlockChan,
				PrivKey:     tt.fields.PrivKey,
				ServerUri:   tt.fields.ServerUri,
			}
			got, err := e.vertify(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvmEngine.vertify() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EvmEngine.vertify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineStart(t *testing.T) {
	EE.Setting(&EmitRequest{
		Typ: "setting",
		Val: map[string]interface{}{
			"app_base_path": "/Users/xan/misc",
			"priv_key":      "37d15846af852c47649005fd6dfe33483d394aaa60c14e7c56f4deadb116329e", //2d09d9f4e166f35a4ab0a2edd599e2a23bbe86b312b2e05b34d9fbe5693b1e48
			"server_uri":    "ws://127.0.0.1:20013",
			"account":       "0x588639773bc6f163aa262245cda746c120676431", //0x7Ac869Ff8b6232f7cfC4370A2df4a81641Cba3d9
		},
	})
	if err := EE.Start(); err != nil {
		t.Error(err)
	}
	select {}
}

/*
0xb1fd8daE392D8E7c5aA0c8D405EFC56e1F55c42c
732d6346fc020087ac84bd1a42adb444e0f783fe907759ba7fa977aab3bf66fc
*/
func TestEmitComplexEmulation(t *testing.T) {
	EE.Setting(&EmitRequest{
		Typ: "setting",
		Val: map[string]interface{}{
			"app_base_path": "/Users/mac/misc",
			"priv_key":      "de4b76c3dca3d8e10aea7644f77b316a68a6476fbd119d441ead5c6131aa42a7", //2d09d9f4e166f35a4ab0a2edd599e2a23bbe86b312b2e05b34d9fbe5693b1e48
			"server_uri":    "ws://54.175.247.94:20013",
			"account":       "0x65081DBA9E7B5398ec4d40f1794003c54dB11B79", //0x7Ac869Ff8b6232f7cfC4370A2df4a81641Cba3d9
		},
	})
	if s := Emit(`{"type":"start"}`); len(s) != 0 {
		fmt.Println(s)
	}

	// i := 0
	// for {
	// 	if s := Emit(`{"type":"state"}`); len(s) != 0 {
	// 		fmt.Println(s)
	// 	}
	// 	<-time.After(1 * time.Second)
	// 	if i%20 == 0 {
	// 		Emit(`{"type":"start"}`)
	// 	}
	// 	i++
	// }

	select {}
}

func TestEngineState(t *testing.T) {
	EE.Setting(&EmitRequest{
		Typ: "setting",
		Val: map[string]interface{}{
			"app_base_path": "/Users/xan/misc",
			"priv_key":      "37d15846af852c47649005fd6dfe33483d394aaa60c14e7c56f4deadb116329e",
			"server_uri":    "ws://127.0.0.1:20013",
			"account":       "0x7Ac869Ff8b6232f7cfC4370A2df4a81641Cba3d9",
		},
	})
	if ss := EE.State(); ss != "stopped" {
		t.Error("unstopped")
	}
	if err := EE.Start(); err != nil {
		t.Error(err)
	}
	if ss := EE.State(); ss != "started" {
		t.Error("unstarted")
	}
	select {}
}

func TestUnmarshalInput(t *testing.T) {
	EE.Setting(&EmitRequest{
		Typ: "setting",
		Val: map[string]interface{}{
			"app_base_path": "/Users/xan/misc",
			"priv_key":      "37d15846af852c47649005fd6dfe33483d394aaa60c14e7c56f4deadb116329e",
			"server_uri":    "ws://127.0.0.1:20013",
			"account":       "0x7Ac869Ff8b6232f7cfC4370A2df4a81641Cba3d9",
		},
	})
	if ss := EE.State(); ss != "stopped" {
		t.Error("unstopped")
	}
	if err := EE.Start(); err != nil {
		t.Error(err)
	}
	if ss := EE.State(); ss != "started" {
		t.Error("unstarted")
	}
	select {}
}

func TestBlssign(t *testing.T) {
	resp := Emit(`
{"type": "blssign","val": {"priv_key": "202cbf36864a88e348c8f573aa0bc79f5a7119e58251c3580bb08af70cb2dfed", "msg" : "8ac7230489e80000"}}
	`)
	_ = resp
}

func TestEngineStop(t *testing.T) {
	if err := EE.Stop(); err != nil {
		t.Error(err)
	}
}

/*
{
    "jsonrpc": "2.0",
    "method": "eth_subscription",
    "params": {
        "subscription": "0x7e6684b10d6c906c47253690b8e9c7fa",
        "result": {
            "Entire": {
                "entire": {
                    "header": {
                        "parentHash": "0x76b286a66229e34455f5238b9f9c0d516903c222b4e5874a6662820e25dcde98",
                        "sha3Uncles": "0x0000000000000000000000000000000000000000000000000000000000000000",
                        "miner": "0x0000000000000000000000000000000000000000",
                        "stateRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
                        "transactionsRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
                        "receiptsRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
                        "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
                        "difficulty": "0x2",
                        "number": "0xd2",
                        "gasLimit": "0x18462971",
                        "gasUsed": "0x0",
                        "timestamp": "0x63a15be4",
                        "extraData": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
                        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
                        "nonce": "0x0000000000000000",
                        "baseFeePerGas": null,
                        "hash": "0x051510ec6c9998e61126d285169ed6b24114b27c264ce0e5dad5e0a7bbac2f42"
                    },
                    "uncles": null,
                    "transactions": [],
                    "snap": {
                        "items": [
                            {
                                "key": "uelEd/X4i16Noul+hQbW5PzwTls=",
                                "value": "CAEaEzB4NjgxNTVhNDM2NzZlMDAwMDAiIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAKiDF0kYBhvcjPJJ+fbLcxwPA5QC2U8qCJzt7+tgEXYWkcA=="
                            },
                            {
                                "key": "m6M2g1Qiuu/FN9dWQpWdKoZlAKM=",
                                "value": "CAEaEzB4NjgxNTVhNDM2NzZlMDAwMDAiIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAKiDF0kYBhvcjPJJ+fbLcxwPA5QC2U8qCJzt7+tgEXYWkcA=="
                            },
                            {
                                "key": "RUH8HMtOBCo7rf5GkE+dItEntoI=",
                                "value": "CAEaEzB4NjgxNTVhNDM2NzZlMDAwMDAiIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAKiDF0kYBhvcjPJJ+fbLcxwPA5QC2U8qCJzt7+tgEXYWkcA=="
                            }
                        ],
                        "outHash": "0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
                    },
                    "proof": "0x0000000000000000000000000000000000000000000000000000000000000000",
                    "senders": null
                },
                "codes": [],
                "headers": [],
                "rewards": {
                    "0x4541fc1ccb4e042a3badfe46904f9d22d127b682": {
                        "Int": "0x20f5b1eaad8d80000"
                    },
                    "0x9ba336835422baefc537d75642959d2a866500a3": {
                        "Int": "0x1d7d843dc3b480000"
                    },
                    "0xb9e94477f5f88b5e8da2e97e8506d6e4fcf04e5b": {
                        "Int": "0x1bc16d674ec800000"
                    }
                }
            }
        }
    }
}
*/

func TestBeanchMark(t *testing.T) {

	var (
		key, _ = crypto.HexToECDSA("d6d8d19bd786d6676819b806694b1100a4414a94e51e9a82a351bd8f7f3f3658")
		addr   = crypto.PubkeyToAddress(key.PublicKey)
		//signer  = new(types.HomesteadSigner)
		//content = context.Background()
	)

	fmt.Println(addr)
}

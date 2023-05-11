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
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/amazechain/amc/common/crypto"
	"github.com/amazechain/amc/common/crypto/ecies"
	"github.com/holiman/uint256"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	commTyp "github.com/amazechain/amc/common/types"
	"github.com/amazechain/amc/log"
	"github.com/amazechain/amc/modules/state"
	"github.com/go-kit/kit/transport/http/jsonrpc"
	"github.com/gorilla/websocket"

	"github.com/amazechain/amc/common/crypto/bls"
)

const (
	VERSION            = ""
	EngineStateRunning = "running"
	EngineStateStopped = "stopped"

	RequestPath = ""
	ConfigPath  = "evm"
	LogFile     = "vertification_debug_log.txt"
)

var (
	IS_DEBUG = true
)

var EE *EvmEngine = &EvmEngine{}

func Emit(jsonText string) string {
	defer func() {
		if err := recover(); err != nil {
			simpleLog("engine panic,err=%+v", err)
			if err := EE.Stop(); err != nil {
				simpleLog("engine stop error,err=%+v", err)
			}
		}
	}()
	er := new(EmitResponse)
	eq := new(EmitRequest)
	if err := json.Unmarshal([]byte(jsonText), eq); err != nil {
		er.Code = 1
		er.Message = err.Error()
		return emitJSON(er)
	}

	if eq.Val != nil {
		simpleLog("emit:", eq.Typ, eq.Val)
	} else {
		simpleLog("emit:", eq.Typ)
	}

	switch eq.Typ {
	case "start":
		if err := EE.Start(); err != nil {
			er.Code, er.Message = 1, err.Error()
			if innerErr, ok := err.(*InnerError); ok {
				er.Code = innerErr.Code
				er.Message = innerErr.Msg
			}
			return emitJSON(er)
		}
	case "stop":
		if err := EE.Stop(); err != nil {
			er.Code, er.Message = 1, err.Error()
			return emitJSON(er)
		}
	case "setting":
		if err := EE.Setting(eq); err != nil {
			er.Code, er.Message = 1, err.Error()
			return emitJSON(er)
		}
	case "list":
		data, err := EE.List()
		if err != nil {
			er.Code, er.Message = 1, err.Error()
			return emitJSON(er)
		}
		er.Data = data
	case "blssign":
		data, err := EE.BlsSign(eq.Val)
		if err != nil {
			er.Code, er.Message = 1, err.Error()
			return emitJSON(er)
		}
		er.Data = data
	case "blspubk":
		data, err := EE.BlsPublicKey(eq.Val)
		if err != nil {
			er.Code, er.Message = 1, err.Error()
			return emitJSON(er)
		}
		er.Data = data
	case "state":
		data := EE.State()
		er.Data = data
	case "decrypt":
		data, err := EE.Decrypt(eq)
		if err != nil {
			er.Code, er.Message = 1, err.Error()
			return emitJSON(er)
		}
		er.Data = data
	case "encrypt":
		data, err := EE.Encrypt(eq)
		if err != nil {
			er.Code, er.Message = 1, err.Error()
			return emitJSON(er)
		}
		er.Data = data
	case "test":
		er.Message = Test()
	}

	return emitJSON(er)
}

func emitJSON(j interface{}) string {
	a, _ := json.Marshal(j)
	return string(a)
}

func (e *EvmEngine) initLogger(lvl string) {
	IS_DEBUG = lvl == "debug"
	if len(lvl) != 0 {
		ensureLogFileExisted()
		log.InitMobileLogger(filepath.Join(EE.AppBasePath, LogFile), IS_DEBUG)
	}
}

func ensureLogFileExisted() {
	logFilePath := path.Join(EE.AppBasePath, LogFile)
	f, err := os.Open(logFilePath)
	if err != nil {
		f, err = os.Create(logFilePath)
		if err != nil {
			fmt.Printf("create log file error,err=%+v\n", err)
			return
		}
	}
	if err = f.Close(); err != nil {
		fmt.Printf("log file close error,err=%+v", err)
	}
}

func simpleLog(txt string, params ...interface{}) {
	if IS_DEBUG {
		ifces := make([]interface{}, 0, len(params)+1)
		ifces = append(ifces, txt)
		ifces = append(ifces, params...)
		fmt.Println(ifces...)
		log.Debug(txt, params...)
	}
}

func simpleLogf(txt string, params ...interface{}) {
	if IS_DEBUG {
		fmt.Printf(txt+"\n", params...)
		log.Debugf(txt, params...)
	}
}

type Setting struct {
	Height      int
	AppBasePath string
	Account     string

	PrivKey string
}

type EmitRequest struct {
	Typ string      `json:"type"`
	Val interface{} `json:"val"`
}
type EmitResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type EvmEngine struct {
	mu         sync.Mutex         `json:"-"`
	ctx        context.Context    `json:"-"`
	cancelFunc context.CancelFunc `json:"-"`
	errChan    chan []error       `json:"-"`

	Account     string      `json:"account"`
	AppBasePath string      `json:"app_base_path"`
	EngineState string      `json:"state"`
	BlockChan   chan string `json:"-"`
	PrivKey     string      `json:"priv_key"`
	ServerUri   string      `json:"server_uri"`
	LogLevel    string      `json:"log_level"`
}

type AccountReward struct {
	Account string
	Number  *uint256.Int
	Value   *uint256.Int
}
type AccountRewards []*AccountReward

func (r AccountRewards) Len() int {
	return len(r)
}

func (r AccountRewards) Less(i, j int) bool {
	return strings.Compare(r[i].Account, r[j].Account) > 0
}

func (r AccountRewards) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

type InnerError struct {
	Code int
	Msg  string
}

func (i InnerError) Error() string { return fmt.Sprintf("code:%+v,msg:%+v", i.Code, i.Msg) }

type AggSign struct {
	Number    uint64            `json:"number"`
	StateRoot commTyp.Hash      `json:"stateRoot"`
	Sign      commTyp.Signature `json:"sign"`
	PublicKey commTyp.PublicKey `json:"publicKey"`
	Address   commTyp.Address   `json:"address"`
}

func (e *EvmEngine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ctx, e.cancelFunc = context.WithCancel(context.Background())
	if e.EngineState == EngineStateRunning {
		return fmt.Errorf("evme is running")
	}
	e.EngineState = EngineStateRunning

	if err := e.verificationTaskBg(); err != nil {
		e.EngineState = EngineStateStopped
		simpleLogf("launch verification task failed,err=%+v", err.Error())
		return err
	}
	return nil
}

func (e *EvmEngine) Stop() error {
	e.mu.Lock()
	e.EngineState = EngineStateStopped
	if e.cancelFunc != nil {
		e.cancelFunc()
	}
	if e.ctx != nil {
		e.ctx.Done()
	}
	e.mu.Unlock()
	return nil
}

func (e *EvmEngine) State() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.EngineState == EngineStateRunning {
		return "started"
	} else {
		return "stopped"
	}
}

/*
	{
		"type":"setting",
		"val":{
			"app_base_path":"/sd/0/evm/",
			"account":"abcdef1234"
		}
	}
*/
func (e *EvmEngine) Setting(req *EmitRequest) error {
	m, ok := req.Val.(map[string]interface{})
	if !ok {
		return fmt.Errorf("type assert error,any==>smap")
	}

	appBasePath, ok := m["app_base_path"]
	if ok {
		if appBasePathStr, ok := appBasePath.(string); ok {
			e.mu.Lock()
			e.AppBasePath = appBasePathStr
			e.mu.Unlock()
		}
	}
	if privKeyIfce, ok := m["priv_key"]; ok {
		if privKey, ok := privKeyIfce.(string); ok {
			e.PrivKey = privKey
		}
	}
	if serverUriIfce, ok := m["server_uri"]; ok {
		if serverUri, ok := serverUriIfce.(string); ok {
			e.ServerUri = serverUri
		}
	}
	if accIfce, ok := m["account"]; ok {
		if account, ok := accIfce.(string); ok {
			e.Account = account
		}
	}

	if logLvlIfce, ok := m["log_level"]; ok {
		if loglvlStr, ok := logLvlIfce.(string); ok {
			e.initLogger(loglvlStr)
		}
	}

	return nil
}
func (e *EvmEngine) List() (interface{}, error) {
	type l struct {
		A string `json:"a"`
		B bool   `json:"b"`
		C int    `json:"c"`
	}

	ll := []*l{
		{
			A: "a",
			B: true,
			C: 123,
		}, {
			A: "aa",
			B: false,
			C: 456,
		},
	}
	return ll, nil
}
func (e *EvmEngine) SaveFile() error {
	return nil
}
func (e *EvmEngine) ReloadFile() error {
	return nil
}
func (e *EvmEngine) BlsSign(req interface{}) (interface{}, error) {
	var privKey, msg string
	blsSignMap, ok := req.(map[string]interface{})
	if !ok {
		return nil, errors.New("empty input value")
	}
	if privKeyIfce, ok := blsSignMap["priv_key"]; ok {
		if privKey, ok = privKeyIfce.(string); !ok {
			return nil, errors.New("priv_key type : array byte")
		}
	} else {
		privKey = e.PrivKey
	}
	if msgIfce, ok := blsSignMap["msg"]; ok {
		if msg, ok = msgIfce.(string); !ok {
			return nil, errors.New("msg type : array byte")
		}
	}
	return BlsSign(privKey, msg)
}
func (e *EvmEngine) BlsPublicKey(req interface{}) (interface{}, error) {
	var privKey string
	blsPublicKeyMap, ok := req.(map[string]interface{})
	if !ok {
		return nil, errors.New("empty input value")
	}
	if privKeyIfce, ok := blsPublicKeyMap["priv_key"]; ok {
		if privKey, ok = privKeyIfce.(string); !ok {
			return nil, errors.New("priv_key type: array byte")
		}
	} else {
		privKey = e.PrivKey
	}
	return BlsPublicKey(privKey)
}

func (e *EvmEngine) verificationTaskBg() error {
	simpleLog("gen pubk")
	pubk, err := BlsPublicKey(e.PrivKey)
	if err != nil {
		simpleLog("generate public key error,", err)
		return err
	}
	simpleLog("init websocket")
	wssvr, err := NewWebSocketService(e.ServerUri, e.Account)
	if err != nil {
		return err
	}
	simpleLog("init websocket chats")
	ochan, ichan, err := wssvr.Chans(pubk.(string))
	if err != nil {
		return err
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				buf := make([]byte, 4096)
				runtime.Stack(buf, true)
				simpleLog("vertification task down", "err", err)
				simpleLog(string(buf))
				// simpleLogf("vertification task down,err=%+v,stk:%s:%d", err, f, l)
			}
			e.mu.Lock()
			e.EngineState = EngineStateStopped
			e.mu.Unlock()
		}()

		for {
			select {
			case <-e.ctx.Done():
				simpleLog("task has been cancelled")
				return
			case msg, ok := <-ochan:
				if !ok {
					simpleLog("task closed")
					return
				}
				entire, err := e.unwrapJSONRPC(msg)
				if err != nil {
					simpleLog("unwrap jsonrpc message error,err=", err)
					continue
				}
				resp, err := e.vertify(entire)
				if err != nil {
					simpleLog("ee verification failed", err)
					continue
				}
				ichan <- resp
			}
		}
	}()
	return nil
}

func (e *EvmEngine) Decrypt(req *EmitRequest) (interface{}, error) {

	var (
		params              map[string]interface{}
		privateKeyInterface interface{}
		privateKeyString    string
		privateKey          *ecdsa.PrivateKey
		messageInterface    interface{}
		message             string
		messageBytes        []byte
		ok                  bool
		err                 error
	)

	if params, ok = req.Val.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("empty input value")
	}

	if privateKeyInterface, ok = params["priv_key"]; !ok {
		return nil, fmt.Errorf("privateKey is empty")
	}

	if privateKeyString, ok = privateKeyInterface.(string); !ok {
		return nil, fmt.Errorf("privateKey type is not string")
	}

	if privateKey, err = crypto.HexToECDSA(privateKeyString); err != nil {
		return nil, fmt.Errorf("cannot decode private key")
	}

	if messageInterface, ok = params["msg"]; !ok {
		return nil, fmt.Errorf("msg is empty")
	}
	if message, ok = messageInterface.(string); !ok {
		return nil, fmt.Errorf("msg type is not string")
	}
	if messageBytes, err = hex.DecodeString(message); err != nil {
		return nil, fmt.Errorf("msg cannote decode to bytes")
	}

	priKey := ecies.ImportECDSA(privateKey)

	ms, err := priKey.Decrypt(messageBytes, nil, nil)
	return hex.EncodeToString(ms), err
}

func (e *EvmEngine) Encrypt(req *EmitRequest) (interface{}, error) {

	var (
		params             map[string]interface{}
		publicKeyInterface interface{}
		publicKeyString    string
		publicKeyBytes     []byte
		publicKey          *ecdsa.PublicKey
		messageInterface   interface{}
		message            string
		messageBytes       []byte
		ok                 bool
		err                error
	)

	if params, ok = req.Val.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("empty input value")
	}

	if publicKeyInterface, ok = params["public_key"]; !ok {
		return nil, fmt.Errorf("public_key is empty")
	}

	if publicKeyString, ok = publicKeyInterface.(string); !ok {
		return nil, fmt.Errorf("public_key type is not string")
	}

	if publicKeyBytes, err = hex.DecodeString(publicKeyString); err != nil {
		return nil, fmt.Errorf("public_key cannote decode to bytes")
	}

	if publicKey, err = crypto.DecompressPubkey(publicKeyBytes); err != nil {
		return nil, fmt.Errorf("cannot decode public key")
	}

	if messageInterface, ok = params["msg"]; !ok {
		return nil, fmt.Errorf("msg is empty")
	}
	if message, ok = messageInterface.(string); !ok {
		return nil, fmt.Errorf("msg type is not string")
	}

	if messageBytes, err = hex.DecodeString(message); err != nil {
		return nil, fmt.Errorf("msg cannote decode to bytes")
	}

	pubKey := ecies.ImportECDSAPublic(publicKey)
	ct, err := ecies.Encrypt(rand.Reader, pubKey, messageBytes, nil, nil)

	return hex.EncodeToString(ct), err
}

//type innerEntireCode state.EntireCode
//
//func (h *innerEntireCode) UnmarshalJSON(in []byte) error {
//	m := map[string]json.RawMessage{}
//	if err := json.Unmarshal(in, &m); err != nil {
//		return err
//	}
//
//	if err := json.Unmarshal(m["coinBase"], &h.CoinBase); err != nil {
//		return err
//	}
//	if err := json.Unmarshal(m["codes"], &h.Codes); err != nil {
//		return err
//	}
//	if rewardsBytes, ok := m["rewards"]; ok {
//		m := []json.RawMessage{}
//		if err := json.Unmarshal(rewardsBytes, &m); err == nil && len(m) != 0 {
//			h.Rewards = make([]block.Reward, len(m))
//			for i, _ := range h.Rewards {
//				rewardBean := block.Reward{
//					Amount: commTyp.NewInt64(0),
//				}
//				json.Unmarshal(m[i], &rewardBean)
//				h.Rewards[i] = rewardBean
//			}
//		}
//	}
//	// if err := json.Unmarshal(m["rewards"], &h.Rewards); err != nil {
//	// 	return err
//	// }
//
//	h.Entire.Header = &block.Header{
//		Difficulty: commTyp.NewInt64(0),
//		Number:     commTyp.NewInt64(0),
//		BaseFee:    commTyp.NewInt64(0),
//	}
//	if err := json.Unmarshal(m["entire"], &h.Entire); err != nil {
//		return err
//	}
//
//	return nil
//}

func (e *EvmEngine) vertify(in []byte) ([]byte, error) {
	var bean state.EntireCode
	if err := json.Unmarshal(in, &bean); err != nil {
		simpleLog("unmarshal vertify input error,err=", err)
		return nil, err
	}

	if bean.Entire.Header == nil {
		return nil, errors.New("nil pointer found")
	}

	entirecode := state.EntireCode(bean)
	stateRoot := verify(e.ctx, &entirecode)

	res := AggSign{}
	//stateroot
	copy(res.StateRoot[:], stateRoot[:])
	if pubkIfce, err := BlsPublicKey(e.PrivKey); err == nil {
		if pubkStr, ok := pubkIfce.(string); ok {
			//publickey
			copy(res.PublicKey[:], []byte(pubkStr))
		}
	}

	simpleLog("==calculated stateroot:", hex.EncodeToString(res.StateRoot[:]))

	//privkey
	res.Number = bean.Entire.Header.Number.Uint64()
	privKeyBytes, err := hex.DecodeString(e.PrivKey)
	if err != nil {
		return nil, err
	}
	arr := [32]byte{}
	copy(arr[:], privKeyBytes[:])
	sk, err := bls.SecretKeyFromRandom32Byte(arr)
	if err != nil {
		return nil, err
	}

	//sign
	copy(res.Sign[:], sk.Sign(res.StateRoot[:]).Marshal())

	//address
	res.Address = commTyp.HexToAddress(e.Account)

	resBytes, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}

	return resBytes, nil
}

func (e *EvmEngine) unwrapJSONRPC(in []byte) ([]byte, error) {
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

//======test methods

func Test() string {
	var sw strings.Builder

	sw.WriteString("version:" + VERSION + "\r\n")

	engJsonBytes, err := json.Marshal(EE)
	if err == nil {
		sw.WriteString(string(engJsonBytes) + "\r\n")
	}

	sw.WriteString("gettime:" + strconv.Itoa(int(GetTime())) + "\r\n")

	sw.WriteString("getapppath:" + GetAppPath() + "\r\n")

	sw.WriteString("getjson:" + GetJson() + "\r\n")

	sw.WriteString("readfile_before:" + ReadTouchedFile() + "\r\n")
	sw.WriteString("touchfile:" + TouchFile() + "\r\n")
	sw.WriteString("readfile_after:" + ReadTouchedFile() + "\r\n")

	ns := GetNetInfos()
	sw.WriteString("getnetinfos:resplen:" + strconv.Itoa(len(ns)) + "\r\n")

	sw.WriteString("backgroundthread:" + BackgroundLoop() + "\r\n")

	sw.WriteString("bls tests " + BlsTest() + "\r\n")

	//block here
	sw.WriteString("wsocket:" + GetWebSocketConnect() + "\r\n")

	return sw.String()
}

func GetTime() int64 {
	return time.Now().Unix()
}

func GetAppPath() string {
	path, err := os.Getwd()
	if err != nil {
		return err.Error()
	}

	return fmt.Sprintf("exec path:%+v;;setting_path:%+v", path, EE.AppBasePath)
}

func GetJson() string {
	return `{
		"a":"b",
		"c":"d"
	}`
}

func TouchFile() string {
	path := path.Join(EE.AppBasePath, "evm_touched_file.txt")
	file, err := os.Create(path)
	if err != nil {
		return err.Error()
	}
	defer file.Close()
	t := time.Now().Unix()
	n, err := file.WriteString(strconv.Itoa(int(t)) + "|NAME=wux PWD=$WORK/src GOOS=android GOARCH=386 CC=$ANDROID_HOME/ndk/23.1.7779620/toolchains/llvm/prebuilt/linux-x86_64/bin/i686-linux-android21-clang CXX=$ANDROID_HOME/ndk/23.1.7779620/toolchains/llvm/prebuilt/linux-x86_64/bin/i686-linux-android21-clang++ CGO_ENABLED=1 GOPATH=$WORK:$GOPATH go mod tidy")
	if err != nil {
		return err.Error()
	}
	if n == 0 {
		return "n==0"
	}
	return file.Name()
}

func ReadTouchedFile() string {
	path := path.Join(EE.AppBasePath, "evm_touched_file.txt")
	file, err := os.Open(path)
	if err != nil {
		return fmt.Sprintln("open touched file error,err=", err.Error())
	}
	defer file.Close()
	allBytes, err := io.ReadAll(file)
	if err != nil {
		return fmt.Sprintln("read touched file error,err=", err.Error())
	}
	return string(allBytes)
}

func GetNetInfos() string {
	resp, err := http.DefaultClient.Get("https://www.baidu.com")
	if err != nil {
		return err.Error()
	}
	htmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err.Error()
	}
	return string(htmlBytes)
}

func RunLoop() string {
	for {
		<-time.After(2 * time.Second)
		fmt.Println("1")
	}
}

var backgroundLoopState = ""

func BackgroundLoop() string {
	if len(backgroundLoopState) != 0 {
		fmt.Println("BackgroundLoop running already")
		return "BackgroundLoop running"
	}
	backgroundLoopState = "123"
	go func() {
		for {
			<-time.After(2 * time.Second)
			fmt.Println("alive")
		}
	}()
	return "OK"
}

func GetWebSocketConnect() string {
	var sw strings.Builder
	conn, connResp, err := websocket.DefaultDialer.Dial("ws://54.175.247.94:20013", nil)
	if err != nil {
		return fmt.Sprintf("dial error,err=%+v \r\n", err)
	}
	defer conn.Close()
	connRespBytes, err := ioutil.ReadAll(connResp.Body)
	if err != nil {
		return fmt.Sprintf("connresp return error,err=%+v", err)
	}
	sw.WriteString(string(connRespBytes))
	err = conn.WriteMessage(websocket.TextMessage, []byte(`{
        "jsonrpc":"2.0",
        "method":"eth_subscribe",
        "params":["newHeads"],
        "id":1
}`))
	if err != nil {
		return fmt.Sprintf("connresp write message error,err=%+v", err)
	}

	_, msg, err := conn.ReadMessage()
	fmt.Println(string(msg))
	if err != nil {
		sw.WriteString("read message error,err=" + err.Error())
	}
	sw.WriteString("received message:" + string(msg))

	go func() {
		innerConn, _, err := websocket.DefaultDialer.Dial("ws://174.129.114.74:8546", nil)
		if err != nil {
			fmt.Printf("bg dial error,err=%+v \r\n", err)
			return
		}
		defer innerConn.Close()
		err = innerConn.WriteMessage(websocket.TextMessage, []byte(`{
			"jsonrpc":"2.0",
			"method":"eth_subscribe",
			"params":["newHeads"],
			"id":1
	}`))
		if err != nil {
			fmt.Printf("bg writemsg error,err=%+v \r\n", err)
			return
		}
		for {
			fmt.Println("bg readmsg:waitting")
			_, msg, err := innerConn.ReadMessage()
			if err != nil {
				fmt.Println("bg readmsg error,err=" + err.Error())
				return
			}
			fmt.Println("bg readed msg:" + string(msg))
		}
	}()

	return sw.String()
}

func BlsTest() string {
	var sw strings.Builder

	var err error
	err = bls.TestSignVerify2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestAggregateVerify2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestAggregateVerify_CompressedSignatures2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestFastAggregateVerify2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestVerifyCompressed2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestMultipleSignatureVerification2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestFastAggregateVerify_ReturnsFalseOnEmptyPubKeyList2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestEth2FastAggregateVerify2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestEth2FastAggregateVerify_ReturnsFalseOnEmptyPubKeyList2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestEth2FastAggregateVerify_ReturnsTrueOnG2PointAtInfinity2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestSignatureFromBytes2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestMultipleSignatureFromBytes2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestCopy2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}
	err = bls.TestSecretKeyFromBytes2()
	if err != nil {
		sw.WriteString(err.Error() + "\r\n")
	}

	sw.WriteString("bls test done.")
	sw.WriteString("==============")

	return sw.String()
}

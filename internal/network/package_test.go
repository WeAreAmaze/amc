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

package network

//func TestHeader(t *testing.T) {
//	var (
//		header  Header
//		msgType message.MessageType
//		msg     []byte
//		buf     *bytes.Buffer
//		payload []byte
//	)
//
//	msgType = message.MsgPingReq
//	msg = []byte("this is a message")
//	buf = new(bytes.Buffer)
//
//	header.Encode(buf, msgType, int32(len(msg)))
//	buf.Write(msg)
//
//	//
//	payload, _ = hexutil.Decode("0x50c402188dcdf09f062a34313244334b6f6f574d716d4c31334b597a5031687a794b43374e72437a35624c335250726163515179635a524150374175463768325f0803125b3059301306072a8648ce3d020106082a8648ce3d030107034200044848ff34cd7de683a77d575281b05000976751ecc205ad246c58a2aabd088c5f12f325dc79cc9f6655712cf6cf9f7493f0a97e7aec4cb8a4ffc75489705f87653a473045022100c4b0fa9f78f2df97776e98f80864cd0aa1eb4fbc8967ade10afca13be892c2b8022047dd6418cb4ff89e2f86e97ec8851da53465951273f69d51f08226d24d8c27e5425c0a0e2f616d632f6170702f312e302e301240356537663532376330386332393664303737363534373466393530326364643338343630373430343535373161633232393236313964653039366564393831351a080a00120410c98706")
//	buf.Reset()
//	buf.Write(payload)
//	// or
//	//payload = buf.Bytes()
//
//	t.Logf("encode message: %s", hexutil.Encode(payload))
//
//	decodeMsgType, remainingLength, err := header.Decode(buf)
//
//	decodeMag, _ := io.ReadAll(buf)
//
//	var protoMessage msg_proto.MessageData
//	if err = proto.Unmarshal(payload, &protoMessage); err != nil {
//		t.Fatalf("can not decode protoMessage err: %s", err)
//	}
//
//	t.Logf("Decode  protoMessage: %v", protoMessage)
//
//	switch decodeMsgType {
//	case message.MsgAppHandshake:
//		var handshakeMessage msg_proto.ProtocolHandshakeMessage
//		if err = proto.Unmarshal(protoMessage.Payload, &handshakeMessage); err != nil {
//			t.Fatalf("can not decode HandshakeMessage err: %s", err)
//		}
//		t.Logf("Decode  HandshakeMessage: %v", handshakeMessage)
//	}
//
//	t.Logf("Decode  message type string:%s, type int:%d, decode len:%d, get len: %d", decodeMsgType, decodeMsgType, remainingLength, len(decodeMag))
//
//	if err != nil {
//		t.Fatalf("can not decode message err: %s", err)
//	}
//
//	if decodeMsgType != msgType {
//		t.Fatalf("decode MsgType err want: %s, get: %s", msgType, decodeMsgType)
//	}
//
//	if remainingLength != int32(len(msg)) {
//		t.Fatalf("decode remainingLength err want: %d, get: %d", int32(len(msg)), remainingLength)
//	}
//
//}

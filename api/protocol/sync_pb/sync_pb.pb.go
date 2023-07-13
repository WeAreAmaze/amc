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

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        v3.20.0
// source: sync_pb.proto

package sync_pb

import (
	types_pb "github.com/amazechain/amc/api/protocol/types_pb"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type BodiesByRangeRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	StartBlockNumber *types_pb.H256 `protobuf:"bytes,1,opt,name=start_block_number,json=startBlockNumber,proto3" json:"start_block_number,omitempty"`
	Count            uint64         `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
	Step             uint64         `protobuf:"varint,3,opt,name=step,proto3" json:"step,omitempty"`
}

func (x *BodiesByRangeRequest) Reset() {
	*x = BodiesByRangeRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sync_pb_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BodiesByRangeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BodiesByRangeRequest) ProtoMessage() {}

func (x *BodiesByRangeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sync_pb_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BodiesByRangeRequest.ProtoReflect.Descriptor instead.
func (*BodiesByRangeRequest) Descriptor() ([]byte, []int) {
	return file_sync_pb_proto_rawDescGZIP(), []int{0}
}

func (x *BodiesByRangeRequest) GetStartBlockNumber() *types_pb.H256 {
	if x != nil {
		return x.StartBlockNumber
	}
	return nil
}

func (x *BodiesByRangeRequest) GetCount() uint64 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *BodiesByRangeRequest) GetStep() uint64 {
	if x != nil {
		return x.Step
	}
	return 0
}

type HeadersByRangeRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	StartBlockNumber *types_pb.H256 `protobuf:"bytes,1,opt,name=start_block_number,json=startBlockNumber,proto3" json:"start_block_number,omitempty"`
	Count            uint64         `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
	Step             uint64         `protobuf:"varint,3,opt,name=step,proto3" json:"step,omitempty"`
}

func (x *HeadersByRangeRequest) Reset() {
	*x = HeadersByRangeRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sync_pb_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HeadersByRangeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HeadersByRangeRequest) ProtoMessage() {}

func (x *HeadersByRangeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sync_pb_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HeadersByRangeRequest.ProtoReflect.Descriptor instead.
func (*HeadersByRangeRequest) Descriptor() ([]byte, []int) {
	return file_sync_pb_proto_rawDescGZIP(), []int{1}
}

func (x *HeadersByRangeRequest) GetStartBlockNumber() *types_pb.H256 {
	if x != nil {
		return x.StartBlockNumber
	}
	return nil
}

func (x *HeadersByRangeRequest) GetCount() uint64 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *HeadersByRangeRequest) GetStep() uint64 {
	if x != nil {
		return x.Step
	}
	return 0
}

// todo
type Ping struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SeqNumber uint64 `protobuf:"varint,1,opt,name=seq_number,json=seqNumber,proto3" json:"seq_number,omitempty"`
}

func (x *Ping) Reset() {
	*x = Ping{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sync_pb_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Ping) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Ping) ProtoMessage() {}

func (x *Ping) ProtoReflect() protoreflect.Message {
	mi := &file_sync_pb_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Ping.ProtoReflect.Descriptor instead.
func (*Ping) Descriptor() ([]byte, []int) {
	return file_sync_pb_proto_rawDescGZIP(), []int{2}
}

func (x *Ping) GetSeqNumber() uint64 {
	if x != nil {
		return x.SeqNumber
	}
	return 0
}

// v2
type Status struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// string version = 1;
	GenesisHash   *types_pb.H256 `protobuf:"bytes,1,opt,name=genesisHash,proto3" json:"genesisHash,omitempty"`
	CurrentHeight *types_pb.H256 `protobuf:"bytes,2,opt,name=currentHeight,proto3" json:"currentHeight,omitempty"`
}

func (x *Status) Reset() {
	*x = Status{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sync_pb_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Status) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Status) ProtoMessage() {}

func (x *Status) ProtoReflect() protoreflect.Message {
	mi := &file_sync_pb_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Status.ProtoReflect.Descriptor instead.
func (*Status) Descriptor() ([]byte, []int) {
	return file_sync_pb_proto_rawDescGZIP(), []int{3}
}

func (x *Status) GetGenesisHash() *types_pb.H256 {
	if x != nil {
		return x.GenesisHash
	}
	return nil
}

func (x *Status) GetCurrentHeight() *types_pb.H256 {
	if x != nil {
		return x.CurrentHeight
	}
	return nil
}

type ForkData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CurrentVersion        *types_pb.H256 `protobuf:"bytes,1,opt,name=current_version,json=currentVersion,proto3" json:"current_version,omitempty"`
	GenesisValidatorsRoot *types_pb.H256 `protobuf:"bytes,2,opt,name=genesis_validators_root,json=genesisValidatorsRoot,proto3" json:"genesis_validators_root,omitempty"`
}

func (x *ForkData) Reset() {
	*x = ForkData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sync_pb_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ForkData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ForkData) ProtoMessage() {}

func (x *ForkData) ProtoReflect() protoreflect.Message {
	mi := &file_sync_pb_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ForkData.ProtoReflect.Descriptor instead.
func (*ForkData) Descriptor() ([]byte, []int) {
	return file_sync_pb_proto_rawDescGZIP(), []int{4}
}

func (x *ForkData) GetCurrentVersion() *types_pb.H256 {
	if x != nil {
		return x.CurrentVersion
	}
	return nil
}

func (x *ForkData) GetGenesisValidatorsRoot() *types_pb.H256 {
	if x != nil {
		return x.GenesisValidatorsRoot
	}
	return nil
}

var File_sync_pb_proto protoreflect.FileDescriptor

var file_sync_pb_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x73, 0x79, 0x6e, 0x63, 0x5f, 0x70, 0x62, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x07, 0x73, 0x79, 0x6e, 0x63, 0x5f, 0x62, 0x70, 0x1a, 0x14, 0x74, 0x79, 0x70, 0x65, 0x73, 0x5f,
	0x70, 0x62, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x7e,
	0x0a, 0x14, 0x42, 0x6f, 0x64, 0x69, 0x65, 0x73, 0x42, 0x79, 0x52, 0x61, 0x6e, 0x67, 0x65, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x3c, 0x0a, 0x12, 0x73, 0x74, 0x61, 0x72, 0x74, 0x5f,
	0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x5f, 0x70, 0x62, 0x2e, 0x48, 0x32,
	0x35, 0x36, 0x52, 0x10, 0x73, 0x74, 0x61, 0x72, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x4e, 0x75,
	0x6d, 0x62, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x04, 0x52, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x74,
	0x65, 0x70, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x73, 0x74, 0x65, 0x70, 0x22, 0x7f,
	0x0a, 0x15, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73, 0x42, 0x79, 0x52, 0x61, 0x6e, 0x67, 0x65,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x3c, 0x0a, 0x12, 0x73, 0x74, 0x61, 0x72, 0x74,
	0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x5f, 0x70, 0x62, 0x2e, 0x48,
	0x32, 0x35, 0x36, 0x52, 0x10, 0x73, 0x74, 0x61, 0x72, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x4e,
	0x75, 0x6d, 0x62, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x73,
	0x74, 0x65, 0x70, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x73, 0x74, 0x65, 0x70, 0x22,
	0x25, 0x0a, 0x04, 0x50, 0x69, 0x6e, 0x67, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x65, 0x71, 0x5f, 0x6e,
	0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x73, 0x65, 0x71,
	0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x22, 0x70, 0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73,
	0x12, 0x30, 0x0a, 0x0b, 0x67, 0x65, 0x6e, 0x65, 0x73, 0x69, 0x73, 0x48, 0x61, 0x73, 0x68, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x5f, 0x70, 0x62,
	0x2e, 0x48, 0x32, 0x35, 0x36, 0x52, 0x0b, 0x67, 0x65, 0x6e, 0x65, 0x73, 0x69, 0x73, 0x48, 0x61,
	0x73, 0x68, 0x12, 0x34, 0x0a, 0x0d, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x48, 0x65, 0x69,
	0x67, 0x68, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x74, 0x79, 0x70, 0x65,
	0x73, 0x5f, 0x70, 0x62, 0x2e, 0x48, 0x32, 0x35, 0x36, 0x52, 0x0d, 0x63, 0x75, 0x72, 0x72, 0x65,
	0x6e, 0x74, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74, 0x22, 0x8b, 0x01, 0x0a, 0x08, 0x46, 0x6f, 0x72,
	0x6b, 0x44, 0x61, 0x74, 0x61, 0x12, 0x37, 0x0a, 0x0f, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74,
	0x5f, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e,
	0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x5f, 0x70, 0x62, 0x2e, 0x48, 0x32, 0x35, 0x36, 0x52, 0x0e,
	0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x46,
	0x0a, 0x17, 0x67, 0x65, 0x6e, 0x65, 0x73, 0x69, 0x73, 0x5f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61,
	0x74, 0x6f, 0x72, 0x73, 0x5f, 0x72, 0x6f, 0x6f, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x0e, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x5f, 0x70, 0x62, 0x2e, 0x48, 0x32, 0x35, 0x36, 0x52,
	0x15, 0x67, 0x65, 0x6e, 0x65, 0x73, 0x69, 0x73, 0x56, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f,
	0x72, 0x73, 0x52, 0x6f, 0x6f, 0x74, 0x42, 0x30, 0x5a, 0x2e, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62,
	0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x61, 0x6d, 0x61, 0x7a, 0x65, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x2f,
	0x61, 0x6d, 0x63, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c,
	0x2f, 0x73, 0x79, 0x6e, 0x63, 0x5f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_sync_pb_proto_rawDescOnce sync.Once
	file_sync_pb_proto_rawDescData = file_sync_pb_proto_rawDesc
)

func file_sync_pb_proto_rawDescGZIP() []byte {
	file_sync_pb_proto_rawDescOnce.Do(func() {
		file_sync_pb_proto_rawDescData = protoimpl.X.CompressGZIP(file_sync_pb_proto_rawDescData)
	})
	return file_sync_pb_proto_rawDescData
}

var file_sync_pb_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_sync_pb_proto_goTypes = []interface{}{
	(*BodiesByRangeRequest)(nil),  // 0: sync_bp.BodiesByRangeRequest
	(*HeadersByRangeRequest)(nil), // 1: sync_bp.HeadersByRangeRequest
	(*Ping)(nil),                  // 2: sync_bp.Ping
	(*Status)(nil),                // 3: sync_bp.Status
	(*ForkData)(nil),              // 4: sync_bp.ForkData
	(*types_pb.H256)(nil),         // 5: types_pb.H256
}
var file_sync_pb_proto_depIdxs = []int32{
	5, // 0: sync_bp.BodiesByRangeRequest.start_block_number:type_name -> types_pb.H256
	5, // 1: sync_bp.HeadersByRangeRequest.start_block_number:type_name -> types_pb.H256
	5, // 2: sync_bp.Status.genesisHash:type_name -> types_pb.H256
	5, // 3: sync_bp.Status.currentHeight:type_name -> types_pb.H256
	5, // 4: sync_bp.ForkData.current_version:type_name -> types_pb.H256
	5, // 5: sync_bp.ForkData.genesis_validators_root:type_name -> types_pb.H256
	6, // [6:6] is the sub-list for method output_type
	6, // [6:6] is the sub-list for method input_type
	6, // [6:6] is the sub-list for extension type_name
	6, // [6:6] is the sub-list for extension extendee
	0, // [0:6] is the sub-list for field type_name
}

func init() { file_sync_pb_proto_init() }
func file_sync_pb_proto_init() {
	if File_sync_pb_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_sync_pb_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BodiesByRangeRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_sync_pb_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HeadersByRangeRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_sync_pb_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Ping); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_sync_pb_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Status); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_sync_pb_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ForkData); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_sync_pb_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_sync_pb_proto_goTypes,
		DependencyIndexes: file_sync_pb_proto_depIdxs,
		MessageInfos:      file_sync_pb_proto_msgTypes,
	}.Build()
	File_sync_pb_proto = out.File
	file_sync_pb_proto_rawDesc = nil
	file_sync_pb_proto_goTypes = nil
	file_sync_pb_proto_depIdxs = nil
}

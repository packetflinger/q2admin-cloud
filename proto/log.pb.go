// compile with:
// protoc --go_out=. --go_opt=paths=source_relative log.proto

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v4.22.2
// source: log.proto

package proto

import (
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

type LogSeverity int32

const (
	LogSeverity_NOT     LogSeverity = 0
	LogSeverity_WARNING LogSeverity = 1 // Meh
	LogSeverity_ERROR   LogSeverity = 2 // Something bad happend
	LogSeverity_FATAL   LogSeverity = 3 // Some serious enough to quit over
	LogSeverity_INFO    LogSeverity = 4 // Related to clients connecting
)

// Enum value maps for LogSeverity.
var (
	LogSeverity_name = map[int32]string{
		0: "NOT",
		1: "WARNING",
		2: "ERROR",
		3: "FATAL",
		4: "INFO",
	}
	LogSeverity_value = map[string]int32{
		"NOT":     0,
		"WARNING": 1,
		"ERROR":   2,
		"FATAL":   3,
		"INFO":    4,
	}
)

func (x LogSeverity) Enum() *LogSeverity {
	p := new(LogSeverity)
	*p = x
	return p
}

func (x LogSeverity) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LogSeverity) Descriptor() protoreflect.EnumDescriptor {
	return file_log_proto_enumTypes[0].Descriptor()
}

func (LogSeverity) Type() protoreflect.EnumType {
	return &file_log_proto_enumTypes[0]
}

func (x LogSeverity) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LogSeverity.Descriptor instead.
func (LogSeverity) EnumDescriptor() ([]byte, []int) {
	return file_log_proto_rawDescGZIP(), []int{0}
}

type LogContext int32

const (
	LogContext_NONE       LogContext = 0
	LogContext_UNKNOWN    LogContext = 1
	LogContext_CONNECTION LogContext = 2
)

// Enum value maps for LogContext.
var (
	LogContext_name = map[int32]string{
		0: "NONE",
		1: "UNKNOWN",
		2: "CONNECTION",
	}
	LogContext_value = map[string]int32{
		"NONE":       0,
		"UNKNOWN":    1,
		"CONNECTION": 2,
	}
)

func (x LogContext) Enum() *LogContext {
	p := new(LogContext)
	*p = x
	return p
}

func (x LogContext) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LogContext) Descriptor() protoreflect.EnumDescriptor {
	return file_log_proto_enumTypes[1].Descriptor()
}

func (LogContext) Type() protoreflect.EnumType {
	return &file_log_proto_enumTypes[1]
}

func (x LogContext) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LogContext.Descriptor instead.
func (LogContext) EnumDescriptor() ([]byte, []int) {
	return file_log_proto_rawDescGZIP(), []int{1}
}

type ServerLog struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Log []*LogEntry `protobuf:"bytes,1,rep,name=log,proto3" json:"log,omitempty"`
}

func (x *ServerLog) Reset() {
	*x = ServerLog{}
	if protoimpl.UnsafeEnabled {
		mi := &file_log_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ServerLog) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServerLog) ProtoMessage() {}

func (x *ServerLog) ProtoReflect() protoreflect.Message {
	mi := &file_log_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ServerLog.ProtoReflect.Descriptor instead.
func (*ServerLog) Descriptor() ([]byte, []int) {
	return file_log_proto_rawDescGZIP(), []int{0}
}

func (x *ServerLog) GetLog() []*LogEntry {
	if x != nil {
		return x.Log
	}
	return nil
}

type LogEntry struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Unix timestamp of when the log entry was emitted
	Time int64 `protobuf:"varint,1,opt,name=time,proto3" json:"time,omitempty"`
	// The UUID of a client for this log entry
	Client string `protobuf:"bytes,2,opt,name=client,proto3" json:"client,omitempty"`
	// How severe are we talking?
	Severity LogSeverity `protobuf:"varint,3,opt,name=severity,proto3,enum=proto.LogSeverity" json:"severity,omitempty"`
	// What is this related to?
	Context LogContext `protobuf:"varint,4,opt,name=context,proto3,enum=proto.LogContext" json:"context,omitempty"`
	// the actual log entry
	Entry string `protobuf:"bytes,5,opt,name=entry,proto3" json:"entry,omitempty"`
}

func (x *LogEntry) Reset() {
	*x = LogEntry{}
	if protoimpl.UnsafeEnabled {
		mi := &file_log_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LogEntry) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LogEntry) ProtoMessage() {}

func (x *LogEntry) ProtoReflect() protoreflect.Message {
	mi := &file_log_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LogEntry.ProtoReflect.Descriptor instead.
func (*LogEntry) Descriptor() ([]byte, []int) {
	return file_log_proto_rawDescGZIP(), []int{1}
}

func (x *LogEntry) GetTime() int64 {
	if x != nil {
		return x.Time
	}
	return 0
}

func (x *LogEntry) GetClient() string {
	if x != nil {
		return x.Client
	}
	return ""
}

func (x *LogEntry) GetSeverity() LogSeverity {
	if x != nil {
		return x.Severity
	}
	return LogSeverity_NOT
}

func (x *LogEntry) GetContext() LogContext {
	if x != nil {
		return x.Context
	}
	return LogContext_NONE
}

func (x *LogEntry) GetEntry() string {
	if x != nil {
		return x.Entry
	}
	return ""
}

var File_log_proto protoreflect.FileDescriptor

var file_log_proto_rawDesc = []byte{
	0x0a, 0x09, 0x6c, 0x6f, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x22, 0x2e, 0x0a, 0x09, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x4c, 0x6f, 0x67, 0x12,
	0x21, 0x0a, 0x03, 0x6c, 0x6f, 0x67, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x4c, 0x6f, 0x67, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x03, 0x6c,
	0x6f, 0x67, 0x22, 0xa9, 0x01, 0x0a, 0x08, 0x4c, 0x6f, 0x67, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12,
	0x12, 0x0a, 0x04, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x04, 0x74,
	0x69, 0x6d, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x12, 0x2e, 0x0a, 0x08, 0x73,
	0x65, 0x76, 0x65, 0x72, 0x69, 0x74, 0x79, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x12, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x4c, 0x6f, 0x67, 0x53, 0x65, 0x76, 0x65, 0x72, 0x69, 0x74,
	0x79, 0x52, 0x08, 0x73, 0x65, 0x76, 0x65, 0x72, 0x69, 0x74, 0x79, 0x12, 0x2b, 0x0a, 0x07, 0x63,
	0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x11, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x4c, 0x6f, 0x67, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x52,
	0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x6e, 0x74, 0x72,
	0x79, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x2a, 0x43,
	0x0a, 0x0b, 0x4c, 0x6f, 0x67, 0x53, 0x65, 0x76, 0x65, 0x72, 0x69, 0x74, 0x79, 0x12, 0x07, 0x0a,
	0x03, 0x4e, 0x4f, 0x54, 0x10, 0x00, 0x12, 0x0b, 0x0a, 0x07, 0x57, 0x41, 0x52, 0x4e, 0x49, 0x4e,
	0x47, 0x10, 0x01, 0x12, 0x09, 0x0a, 0x05, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10, 0x02, 0x12, 0x09,
	0x0a, 0x05, 0x46, 0x41, 0x54, 0x41, 0x4c, 0x10, 0x03, 0x12, 0x08, 0x0a, 0x04, 0x49, 0x4e, 0x46,
	0x4f, 0x10, 0x04, 0x2a, 0x33, 0x0a, 0x0a, 0x4c, 0x6f, 0x67, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78,
	0x74, 0x12, 0x08, 0x0a, 0x04, 0x4e, 0x4f, 0x4e, 0x45, 0x10, 0x00, 0x12, 0x0b, 0x0a, 0x07, 0x55,
	0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e, 0x10, 0x01, 0x12, 0x0e, 0x0a, 0x0a, 0x43, 0x4f, 0x4e, 0x4e,
	0x45, 0x43, 0x54, 0x49, 0x4f, 0x4e, 0x10, 0x02, 0x42, 0x29, 0x5a, 0x27, 0x67, 0x69, 0x74, 0x68,
	0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x61, 0x63, 0x6b, 0x65, 0x74, 0x66, 0x6c, 0x69,
	0x6e, 0x67, 0x65, 0x72, 0x2f, 0x71, 0x32, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x64, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_log_proto_rawDescOnce sync.Once
	file_log_proto_rawDescData = file_log_proto_rawDesc
)

func file_log_proto_rawDescGZIP() []byte {
	file_log_proto_rawDescOnce.Do(func() {
		file_log_proto_rawDescData = protoimpl.X.CompressGZIP(file_log_proto_rawDescData)
	})
	return file_log_proto_rawDescData
}

var file_log_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_log_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_log_proto_goTypes = []interface{}{
	(LogSeverity)(0),  // 0: proto.LogSeverity
	(LogContext)(0),   // 1: proto.LogContext
	(*ServerLog)(nil), // 2: proto.ServerLog
	(*LogEntry)(nil),  // 3: proto.LogEntry
}
var file_log_proto_depIdxs = []int32{
	3, // 0: proto.ServerLog.log:type_name -> proto.LogEntry
	0, // 1: proto.LogEntry.severity:type_name -> proto.LogSeverity
	1, // 2: proto.LogEntry.context:type_name -> proto.LogContext
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_log_proto_init() }
func file_log_proto_init() {
	if File_log_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_log_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ServerLog); i {
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
		file_log_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LogEntry); i {
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
			RawDescriptor: file_log_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_log_proto_goTypes,
		DependencyIndexes: file_log_proto_depIdxs,
		EnumInfos:         file_log_proto_enumTypes,
		MessageInfos:      file_log_proto_msgTypes,
	}.Build()
	File_log_proto = out.File
	file_log_proto_rawDesc = nil
	file_log_proto_goTypes = nil
	file_log_proto_depIdxs = nil
}

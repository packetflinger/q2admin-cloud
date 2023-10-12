// compile with:
// protoc --go_out=. --go_opt=paths=source_relative *.proto

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v4.22.2
// source: config.proto

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

type Config struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address         string `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"` // ip addr
	Port            uint32 `protobuf:"varint,2,opt,name=port,proto3" json:"port,omitempty"`
	Database        string `protobuf:"bytes,3,opt,name=database,proto3" json:"database,omitempty"`                       // sqlite file
	PrivateKey      string `protobuf:"bytes,4,opt,name=private_key,json=privateKey,proto3" json:"private_key,omitempty"` // path
	ApiEnabled      bool   `protobuf:"varint,5,opt,name=api_enabled,json=apiEnabled,proto3" json:"api_enabled,omitempty"`
	ApiAddress      string `protobuf:"bytes,6,opt,name=api_address,json=apiAddress,proto3" json:"api_address,omitempty"` // ip addr
	ApiPort         uint32 `protobuf:"varint,7,opt,name=api_port,json=apiPort,proto3" json:"api_port,omitempty"`
	ClientFile      string `protobuf:"bytes,8,opt,name=client_file,json=clientFile,proto3" json:"client_file,omitempty"`
	ClientDirectory string `protobuf:"bytes,9,opt,name=client_directory,json=clientDirectory,proto3" json:"client_directory,omitempty"`
	UserFile        string `protobuf:"bytes,10,opt,name=user_file,json=userFile,proto3" json:"user_file,omitempty"`
	AccessFile      string `protobuf:"bytes,11,opt,name=access_file,json=accessFile,proto3" json:"access_file,omitempty"`
	AuthFile        string `protobuf:"bytes,12,opt,name=auth_file,json=authFile,proto3" json:"auth_file,omitempty"`
	MaintenanceTime uint32 `protobuf:"varint,13,opt,name=maintenance_time,json=maintenanceTime,proto3" json:"maintenance_time,omitempty"` // seconds
	DebugMode       bool   `protobuf:"varint,14,opt,name=debug_mode,json=debugMode,proto3" json:"debug_mode,omitempty"`
	WebRoot         string `protobuf:"bytes,15,opt,name=web_root,json=webRoot,proto3" json:"web_root,omitempty"` // where are website file?
}

func (x *Config) Reset() {
	*x = Config{}
	if protoimpl.UnsafeEnabled {
		mi := &file_config_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Config) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Config) ProtoMessage() {}

func (x *Config) ProtoReflect() protoreflect.Message {
	mi := &file_config_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Config.ProtoReflect.Descriptor instead.
func (*Config) Descriptor() ([]byte, []int) {
	return file_config_proto_rawDescGZIP(), []int{0}
}

func (x *Config) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *Config) GetPort() uint32 {
	if x != nil {
		return x.Port
	}
	return 0
}

func (x *Config) GetDatabase() string {
	if x != nil {
		return x.Database
	}
	return ""
}

func (x *Config) GetPrivateKey() string {
	if x != nil {
		return x.PrivateKey
	}
	return ""
}

func (x *Config) GetApiEnabled() bool {
	if x != nil {
		return x.ApiEnabled
	}
	return false
}

func (x *Config) GetApiAddress() string {
	if x != nil {
		return x.ApiAddress
	}
	return ""
}

func (x *Config) GetApiPort() uint32 {
	if x != nil {
		return x.ApiPort
	}
	return 0
}

func (x *Config) GetClientFile() string {
	if x != nil {
		return x.ClientFile
	}
	return ""
}

func (x *Config) GetClientDirectory() string {
	if x != nil {
		return x.ClientDirectory
	}
	return ""
}

func (x *Config) GetUserFile() string {
	if x != nil {
		return x.UserFile
	}
	return ""
}

func (x *Config) GetAccessFile() string {
	if x != nil {
		return x.AccessFile
	}
	return ""
}

func (x *Config) GetAuthFile() string {
	if x != nil {
		return x.AuthFile
	}
	return ""
}

func (x *Config) GetMaintenanceTime() uint32 {
	if x != nil {
		return x.MaintenanceTime
	}
	return 0
}

func (x *Config) GetDebugMode() bool {
	if x != nil {
		return x.DebugMode
	}
	return false
}

func (x *Config) GetWebRoot() string {
	if x != nil {
		return x.WebRoot
	}
	return ""
}

var File_config_proto protoreflect.FileDescriptor

var file_config_proto_rawDesc = []byte{
	0x0a, 0x0c, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xdc, 0x03, 0x0a, 0x06, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x6f,
	0x72, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x1a,
	0x0a, 0x08, 0x64, 0x61, 0x74, 0x61, 0x62, 0x61, 0x73, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x08, 0x64, 0x61, 0x74, 0x61, 0x62, 0x61, 0x73, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x70, 0x72,
	0x69, 0x76, 0x61, 0x74, 0x65, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0a, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x4b, 0x65, 0x79, 0x12, 0x1f, 0x0a, 0x0b, 0x61,
	0x70, 0x69, 0x5f, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x64, 0x18, 0x05, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x0a, 0x61, 0x70, 0x69, 0x45, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x64, 0x12, 0x1f, 0x0a, 0x0b,
	0x61, 0x70, 0x69, 0x5f, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0a, 0x61, 0x70, 0x69, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x19, 0x0a,
	0x08, 0x61, 0x70, 0x69, 0x5f, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0d, 0x52,
	0x07, 0x61, 0x70, 0x69, 0x50, 0x6f, 0x72, 0x74, 0x12, 0x1f, 0x0a, 0x0b, 0x63, 0x6c, 0x69, 0x65,
	0x6e, 0x74, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x63,
	0x6c, 0x69, 0x65, 0x6e, 0x74, 0x46, 0x69, 0x6c, 0x65, 0x12, 0x29, 0x0a, 0x10, 0x63, 0x6c, 0x69,
	0x65, 0x6e, 0x74, 0x5f, 0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x79, 0x18, 0x09, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0f, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x44, 0x69, 0x72, 0x65, 0x63,
	0x74, 0x6f, 0x72, 0x79, 0x12, 0x1b, 0x0a, 0x09, 0x75, 0x73, 0x65, 0x72, 0x5f, 0x66, 0x69, 0x6c,
	0x65, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x75, 0x73, 0x65, 0x72, 0x46, 0x69, 0x6c,
	0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x61, 0x63, 0x63, 0x65, 0x73, 0x73, 0x5f, 0x66, 0x69, 0x6c, 0x65,
	0x18, 0x0b, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x61, 0x63, 0x63, 0x65, 0x73, 0x73, 0x46, 0x69,
	0x6c, 0x65, 0x12, 0x1b, 0x0a, 0x09, 0x61, 0x75, 0x74, 0x68, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x18,
	0x0c, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x61, 0x75, 0x74, 0x68, 0x46, 0x69, 0x6c, 0x65, 0x12,
	0x29, 0x0a, 0x10, 0x6d, 0x61, 0x69, 0x6e, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x5f, 0x74,
	0x69, 0x6d, 0x65, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x0f, 0x6d, 0x61, 0x69, 0x6e, 0x74,
	0x65, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x64, 0x65,
	0x62, 0x75, 0x67, 0x5f, 0x6d, 0x6f, 0x64, 0x65, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09,
	0x64, 0x65, 0x62, 0x75, 0x67, 0x4d, 0x6f, 0x64, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x77, 0x65, 0x62,
	0x5f, 0x72, 0x6f, 0x6f, 0x74, 0x18, 0x0f, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x77, 0x65, 0x62,
	0x52, 0x6f, 0x6f, 0x74, 0x42, 0x29, 0x5a, 0x27, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63,
	0x6f, 0x6d, 0x2f, 0x70, 0x61, 0x63, 0x6b, 0x65, 0x74, 0x66, 0x6c, 0x69, 0x6e, 0x67, 0x65, 0x72,
	0x2f, 0x71, 0x32, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x64, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_config_proto_rawDescOnce sync.Once
	file_config_proto_rawDescData = file_config_proto_rawDesc
)

func file_config_proto_rawDescGZIP() []byte {
	file_config_proto_rawDescOnce.Do(func() {
		file_config_proto_rawDescData = protoimpl.X.CompressGZIP(file_config_proto_rawDescData)
	})
	return file_config_proto_rawDescData
}

var file_config_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_config_proto_goTypes = []interface{}{
	(*Config)(nil), // 0: proto.Config
}
var file_config_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_config_proto_init() }
func file_config_proto_init() {
	if File_config_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_config_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Config); i {
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
			RawDescriptor: file_config_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_config_proto_goTypes,
		DependencyIndexes: file_config_proto_depIdxs,
		MessageInfos:      file_config_proto_msgTypes,
	}.Build()
	File_config_proto = out.File
	file_config_proto_rawDesc = nil
	file_config_proto_goTypes = nil
	file_config_proto_depIdxs = nil
}

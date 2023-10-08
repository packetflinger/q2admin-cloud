// compile with:
// protoc --go_out=. --go_opt=paths=source_relative *.proto

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v4.22.2
// source: oauth.proto

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

type Credentials_Type int32

const (
	Credentials_GOOGLE  Credentials_Type = 0
	Credentials_DISCORD Credentials_Type = 1
)

// Enum value maps for Credentials_Type.
var (
	Credentials_Type_name = map[int32]string{
		0: "GOOGLE",
		1: "DISCORD",
	}
	Credentials_Type_value = map[string]int32{
		"GOOGLE":  0,
		"DISCORD": 1,
	}
)

func (x Credentials_Type) Enum() *Credentials_Type {
	p := new(Credentials_Type)
	*p = x
	return p
}

func (x Credentials_Type) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Credentials_Type) Descriptor() protoreflect.EnumDescriptor {
	return file_oauth_proto_enumTypes[0].Descriptor()
}

func (Credentials_Type) Type() protoreflect.EnumType {
	return &file_oauth_proto_enumTypes[0]
}

func (x Credentials_Type) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Credentials_Type.Descriptor instead.
func (Credentials_Type) EnumDescriptor() ([]byte, []int) {
	return file_oauth_proto_rawDescGZIP(), []int{0, 0}
}

type Credentials struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Oauth []*Credentials_OAuth `protobuf:"bytes,1,rep,name=oauth,proto3" json:"oauth,omitempty"`
}

func (x *Credentials) Reset() {
	*x = Credentials{}
	if protoimpl.UnsafeEnabled {
		mi := &file_oauth_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Credentials) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Credentials) ProtoMessage() {}

func (x *Credentials) ProtoReflect() protoreflect.Message {
	mi := &file_oauth_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Credentials.ProtoReflect.Descriptor instead.
func (*Credentials) Descriptor() ([]byte, []int) {
	return file_oauth_proto_rawDescGZIP(), []int{0}
}

func (x *Credentials) GetOauth() []*Credentials_OAuth {
	if x != nil {
		return x.Oauth
	}
	return nil
}

type Credentials_OAuth struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type          Credentials_Type `protobuf:"varint,1,opt,name=type,proto3,enum=proto.Credentials_Type" json:"type,omitempty"`
	AuthUrl       string           `protobuf:"bytes,2,opt,name=auth_url,json=authUrl,proto3" json:"auth_url,omitempty"`
	TokenUrl      string           `protobuf:"bytes,3,opt,name=token_url,json=tokenUrl,proto3" json:"token_url,omitempty"`
	ClientId      string           `protobuf:"bytes,4,opt,name=client_id,json=clientId,proto3" json:"client_id,omitempty"`
	Secret        string           `protobuf:"bytes,5,opt,name=secret,proto3" json:"secret,omitempty"`
	Scope         []string         `protobuf:"bytes,6,rep,name=scope,proto3" json:"scope,omitempty"`
	ImagePath     string           `protobuf:"bytes,7,opt,name=image_path,json=imagePath,proto3" json:"image_path,omitempty"`
	AlternateText string           `protobuf:"bytes,8,opt,name=alternate_text,json=alternateText,proto3" json:"alternate_text,omitempty"`
	CallbackUrl   string           `protobuf:"bytes,9,opt,name=callback_url,json=callbackUrl,proto3" json:"callback_url,omitempty"`
	Disabled      bool             `protobuf:"varint,10,opt,name=disabled,proto3" json:"disabled,omitempty"`
}

func (x *Credentials_OAuth) Reset() {
	*x = Credentials_OAuth{}
	if protoimpl.UnsafeEnabled {
		mi := &file_oauth_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Credentials_OAuth) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Credentials_OAuth) ProtoMessage() {}

func (x *Credentials_OAuth) ProtoReflect() protoreflect.Message {
	mi := &file_oauth_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Credentials_OAuth.ProtoReflect.Descriptor instead.
func (*Credentials_OAuth) Descriptor() ([]byte, []int) {
	return file_oauth_proto_rawDescGZIP(), []int{0, 0}
}

func (x *Credentials_OAuth) GetType() Credentials_Type {
	if x != nil {
		return x.Type
	}
	return Credentials_GOOGLE
}

func (x *Credentials_OAuth) GetAuthUrl() string {
	if x != nil {
		return x.AuthUrl
	}
	return ""
}

func (x *Credentials_OAuth) GetTokenUrl() string {
	if x != nil {
		return x.TokenUrl
	}
	return ""
}

func (x *Credentials_OAuth) GetClientId() string {
	if x != nil {
		return x.ClientId
	}
	return ""
}

func (x *Credentials_OAuth) GetSecret() string {
	if x != nil {
		return x.Secret
	}
	return ""
}

func (x *Credentials_OAuth) GetScope() []string {
	if x != nil {
		return x.Scope
	}
	return nil
}

func (x *Credentials_OAuth) GetImagePath() string {
	if x != nil {
		return x.ImagePath
	}
	return ""
}

func (x *Credentials_OAuth) GetAlternateText() string {
	if x != nil {
		return x.AlternateText
	}
	return ""
}

func (x *Credentials_OAuth) GetCallbackUrl() string {
	if x != nil {
		return x.CallbackUrl
	}
	return ""
}

func (x *Credentials_OAuth) GetDisabled() bool {
	if x != nil {
		return x.Disabled
	}
	return false
}

var File_oauth_proto protoreflect.FileDescriptor

var file_oauth_proto_rawDesc = []byte{
	0x0a, 0x0b, 0x6f, 0x61, 0x75, 0x74, 0x68, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0x9d, 0x03, 0x0a, 0x0b, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74,
	0x69, 0x61, 0x6c, 0x73, 0x12, 0x2e, 0x0a, 0x05, 0x6f, 0x61, 0x75, 0x74, 0x68, 0x18, 0x01, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x18, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x43, 0x72, 0x65, 0x64,
	0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x2e, 0x4f, 0x41, 0x75, 0x74, 0x68, 0x52, 0x05, 0x6f,
	0x61, 0x75, 0x74, 0x68, 0x1a, 0xbc, 0x02, 0x0a, 0x05, 0x4f, 0x41, 0x75, 0x74, 0x68, 0x12, 0x2b,
	0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x17, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73,
	0x2e, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x61,
	0x75, 0x74, 0x68, 0x5f, 0x75, 0x72, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61,
	0x75, 0x74, 0x68, 0x55, 0x72, 0x6c, 0x12, 0x1b, 0x0a, 0x09, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x5f,
	0x75, 0x72, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x74, 0x6f, 0x6b, 0x65, 0x6e,
	0x55, 0x72, 0x6c, 0x12, 0x1b, 0x0a, 0x09, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x5f, 0x69, 0x64,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x49, 0x64,
	0x12, 0x16, 0x0a, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x63, 0x6f, 0x70,
	0x65, 0x18, 0x06, 0x20, 0x03, 0x28, 0x09, 0x52, 0x05, 0x73, 0x63, 0x6f, 0x70, 0x65, 0x12, 0x1d,
	0x0a, 0x0a, 0x69, 0x6d, 0x61, 0x67, 0x65, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18, 0x07, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x09, 0x69, 0x6d, 0x61, 0x67, 0x65, 0x50, 0x61, 0x74, 0x68, 0x12, 0x25, 0x0a,
	0x0e, 0x61, 0x6c, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x74, 0x65, 0x5f, 0x74, 0x65, 0x78, 0x74, 0x18,
	0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x61, 0x6c, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x74, 0x65,
	0x54, 0x65, 0x78, 0x74, 0x12, 0x21, 0x0a, 0x0c, 0x63, 0x61, 0x6c, 0x6c, 0x62, 0x61, 0x63, 0x6b,
	0x5f, 0x75, 0x72, 0x6c, 0x18, 0x09, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x63, 0x61, 0x6c, 0x6c,
	0x62, 0x61, 0x63, 0x6b, 0x55, 0x72, 0x6c, 0x12, 0x1a, 0x0a, 0x08, 0x64, 0x69, 0x73, 0x61, 0x62,
	0x6c, 0x65, 0x64, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x64, 0x69, 0x73, 0x61, 0x62,
	0x6c, 0x65, 0x64, 0x22, 0x1f, 0x0a, 0x04, 0x54, 0x79, 0x70, 0x65, 0x12, 0x0a, 0x0a, 0x06, 0x47,
	0x4f, 0x4f, 0x47, 0x4c, 0x45, 0x10, 0x00, 0x12, 0x0b, 0x0a, 0x07, 0x44, 0x49, 0x53, 0x43, 0x4f,
	0x52, 0x44, 0x10, 0x01, 0x42, 0x29, 0x5a, 0x27, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63,
	0x6f, 0x6d, 0x2f, 0x70, 0x61, 0x63, 0x6b, 0x65, 0x74, 0x66, 0x6c, 0x69, 0x6e, 0x67, 0x65, 0x72,
	0x2f, 0x71, 0x32, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x64, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_oauth_proto_rawDescOnce sync.Once
	file_oauth_proto_rawDescData = file_oauth_proto_rawDesc
)

func file_oauth_proto_rawDescGZIP() []byte {
	file_oauth_proto_rawDescOnce.Do(func() {
		file_oauth_proto_rawDescData = protoimpl.X.CompressGZIP(file_oauth_proto_rawDescData)
	})
	return file_oauth_proto_rawDescData
}

var file_oauth_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_oauth_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_oauth_proto_goTypes = []interface{}{
	(Credentials_Type)(0),     // 0: proto.Credentials.Type
	(*Credentials)(nil),       // 1: proto.Credentials
	(*Credentials_OAuth)(nil), // 2: proto.Credentials.OAuth
}
var file_oauth_proto_depIdxs = []int32{
	2, // 0: proto.Credentials.oauth:type_name -> proto.Credentials.OAuth
	0, // 1: proto.Credentials.OAuth.type:type_name -> proto.Credentials.Type
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_oauth_proto_init() }
func file_oauth_proto_init() {
	if File_oauth_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_oauth_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Credentials); i {
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
		file_oauth_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Credentials_OAuth); i {
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
			RawDescriptor: file_oauth_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_oauth_proto_goTypes,
		DependencyIndexes: file_oauth_proto_depIdxs,
		EnumInfos:         file_oauth_proto_enumTypes,
		MessageInfos:      file_oauth_proto_msgTypes,
	}.Build()
	File_oauth_proto = out.File
	file_oauth_proto_rawDesc = nil
	file_oauth_proto_goTypes = nil
	file_oauth_proto_depIdxs = nil
}

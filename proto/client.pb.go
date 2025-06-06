// compile with:
// protoc --go_out=. --go_opt=paths=source_relative client.proto

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v4.22.2
// source: client.proto

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

type DelegateRestriction int32

const (
	DelegateRestriction_DELEGATE_RESTRICTION_NONE     DelegateRestriction = 0 // can do anything
	DelegateRestriction_DELEGATE_RESTRICTION_VIEWONLY DelegateRestriction = 1 // only creep
	DelegateRestriction_DELEGATE_RESTRICTION_CHATONLY DelegateRestriction = 2 // only creep and talk back
)

// Enum value maps for DelegateRestriction.
var (
	DelegateRestriction_name = map[int32]string{
		0: "DELEGATE_RESTRICTION_NONE",
		1: "DELEGATE_RESTRICTION_VIEWONLY",
		2: "DELEGATE_RESTRICTION_CHATONLY",
	}
	DelegateRestriction_value = map[string]int32{
		"DELEGATE_RESTRICTION_NONE":     0,
		"DELEGATE_RESTRICTION_VIEWONLY": 1,
		"DELEGATE_RESTRICTION_CHATONLY": 2,
	}
)

func (x DelegateRestriction) Enum() *DelegateRestriction {
	p := new(DelegateRestriction)
	*p = x
	return p
}

func (x DelegateRestriction) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (DelegateRestriction) Descriptor() protoreflect.EnumDescriptor {
	return file_client_proto_enumTypes[0].Descriptor()
}

func (DelegateRestriction) Type() protoreflect.EnumType {
	return &file_client_proto_enumTypes[0]
}

func (x DelegateRestriction) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use DelegateRestriction.Descriptor instead.
func (DelegateRestriction) EnumDescriptor() ([]byte, []int) {
	return file_client_proto_rawDescGZIP(), []int{0}
}

type Clients struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Client []*Client `protobuf:"bytes,1,rep,name=client,proto3" json:"client,omitempty"`
}

func (x *Clients) Reset() {
	*x = Clients{}
	if protoimpl.UnsafeEnabled {
		mi := &file_client_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Clients) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Clients) ProtoMessage() {}

func (x *Clients) ProtoReflect() protoreflect.Message {
	mi := &file_client_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Clients.ProtoReflect.Descriptor instead.
func (*Clients) Descriptor() ([]byte, []int) {
	return file_client_proto_rawDescGZIP(), []int{0}
}

func (x *Clients) GetClient() []*Client {
	if x != nil {
		return x.Client
	}
	return nil
}

type Client struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// This is just a random unique identifier. Can be anything really as
	// long as it's unique among all clients. Using a v4 uuid is easy way
	// to ensure that.
	//
	// Private to server
	Uuid string `protobuf:"bytes,1,opt,name=uuid,proto3" json:"uuid,omitempty"`
	// A more meaningful identifier, typically teleport name of the server.
	// Should not have spaces or special characters. Good rule of thumb
	// would be all lower case characters and groups separated by dashes.
	//
	// User definable. Examples: pf-tdm-nj, ctf-main, etc
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	// Email address of the user who created it.
	//
	// Private to server
	Owner string `protobuf:"bytes,3,opt,name=owner,proto3" json:"owner,omitempty"`
	// Short blurb about the server, displayed to players when teleport
	// is used.
	//
	// User definable.
	Description string `protobuf:"bytes,4,opt,name=description,proto3" json:"description,omitempty"`
	// The ip:port of the server. Can be either ip address or DNS name
	//
	// Examples: "192.0.2.4:27910", "frag.gr:27999"
	// Raw IPv6 example: "[2001:db8:beef:c0ff:ee]:27933"
	//
	// User definable.
	Address string `protobuf:"bytes,5,opt,name=address,proto3" json:"address,omitempty"`
	// The local file in the client's directory containing the client's
	// public key. This key is an RSA public key, not an SSH public key.
	//
	// Default: "key"
	PublicKey string `protobuf:"bytes,6,opt,name=public_key,json=publicKey,proto3" json:"public_key,omitempty"`
	// The client has been verified by the server. This is in addition
	// to the standard challenge-based authentication.
	Verified bool `protobuf:"varint,7,opt,name=verified,proto3" json:"verified,omitempty"`
	// Players on this client are allowed to use the teleport feature
	AllowTeleport bool `protobuf:"varint,8,opt,name=allow_teleport,json=allowTeleport,proto3" json:"allow_teleport,omitempty"`
	// Players on this client are allowed to use the invite feature
	AllowInvite bool `protobuf:"varint,9,opt,name=allow_invite,json=allowInvite,proto3" json:"allow_invite,omitempty"`
	// The filename used for logging inside the client's directory
	//
	// default: "log"
	LogFile string `protobuf:"bytes,10,opt,name=log_file,json=logFile,proto3" json:"log_file,omitempty"`
	// other people given access
	Delegate []*Delegate `protobuf:"bytes,11,rep,name=delegate,proto3" json:"delegate,omitempty"`
	// any api keys that have been created for this client
	ApiKeys *ApiKeys `protobuf:"bytes,12,opt,name=api_keys,json=apiKeys,proto3" json:"api_keys,omitempty"`
	// permissions
	Access []*ClientAccess `protobuf:"bytes,13,rep,name=access,proto3" json:"access,omitempty"`
	// whether the server should ignore this client
	Disabled bool `protobuf:"varint,14,opt,name=disabled,proto3" json:"disabled,omitempty"`
}

func (x *Client) Reset() {
	*x = Client{}
	if protoimpl.UnsafeEnabled {
		mi := &file_client_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Client) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Client) ProtoMessage() {}

func (x *Client) ProtoReflect() protoreflect.Message {
	mi := &file_client_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Client.ProtoReflect.Descriptor instead.
func (*Client) Descriptor() ([]byte, []int) {
	return file_client_proto_rawDescGZIP(), []int{1}
}

func (x *Client) GetUuid() string {
	if x != nil {
		return x.Uuid
	}
	return ""
}

func (x *Client) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Client) GetOwner() string {
	if x != nil {
		return x.Owner
	}
	return ""
}

func (x *Client) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *Client) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *Client) GetPublicKey() string {
	if x != nil {
		return x.PublicKey
	}
	return ""
}

func (x *Client) GetVerified() bool {
	if x != nil {
		return x.Verified
	}
	return false
}

func (x *Client) GetAllowTeleport() bool {
	if x != nil {
		return x.AllowTeleport
	}
	return false
}

func (x *Client) GetAllowInvite() bool {
	if x != nil {
		return x.AllowInvite
	}
	return false
}

func (x *Client) GetLogFile() string {
	if x != nil {
		return x.LogFile
	}
	return ""
}

func (x *Client) GetDelegate() []*Delegate {
	if x != nil {
		return x.Delegate
	}
	return nil
}

func (x *Client) GetApiKeys() *ApiKeys {
	if x != nil {
		return x.ApiKeys
	}
	return nil
}

func (x *Client) GetAccess() []*ClientAccess {
	if x != nil {
		return x.Access
	}
	return nil
}

func (x *Client) GetDisabled() bool {
	if x != nil {
		return x.Disabled
	}
	return false
}

// who can access this client and how
type ClientAccess struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	User  *User   `protobuf:"bytes,1,opt,name=user,proto3" json:"user,omitempty"`
	Roles []*Role `protobuf:"bytes,2,rep,name=roles,proto3" json:"roles,omitempty"`
}

func (x *ClientAccess) Reset() {
	*x = ClientAccess{}
	if protoimpl.UnsafeEnabled {
		mi := &file_client_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ClientAccess) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientAccess) ProtoMessage() {}

func (x *ClientAccess) ProtoReflect() protoreflect.Message {
	mi := &file_client_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClientAccess.ProtoReflect.Descriptor instead.
func (*ClientAccess) Descriptor() ([]byte, []int) {
	return file_client_proto_rawDescGZIP(), []int{2}
}

func (x *ClientAccess) GetUser() *User {
	if x != nil {
		return x.User
	}
	return nil
}

func (x *ClientAccess) GetRoles() []*Role {
	if x != nil {
		return x.Roles
	}
	return nil
}

// A client delegate is a user other than the owner (creator) who has access
// to the client. This user will see this client in their /my-servers page.
type Delegate struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// the user's email address
	Identity string `protobuf:"bytes,1,opt,name=identity,proto3" json:"identity,omitempty"`
	// the restriction applied to this context
	Restriction DelegateRestriction `protobuf:"varint,2,opt,name=restriction,proto3,enum=proto.DelegateRestriction" json:"restriction,omitempty"`
}

func (x *Delegate) Reset() {
	*x = Delegate{}
	if protoimpl.UnsafeEnabled {
		mi := &file_client_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Delegate) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Delegate) ProtoMessage() {}

func (x *Delegate) ProtoReflect() protoreflect.Message {
	mi := &file_client_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Delegate.ProtoReflect.Descriptor instead.
func (*Delegate) Descriptor() ([]byte, []int) {
	return file_client_proto_rawDescGZIP(), []int{3}
}

func (x *Delegate) GetIdentity() string {
	if x != nil {
		return x.Identity
	}
	return ""
}

func (x *Delegate) GetRestriction() DelegateRestriction {
	if x != nil {
		return x.Restriction
	}
	return DelegateRestriction_DELEGATE_RESTRICTION_NONE
}

// Used by the main server program to keep track of which clients it should
// load on startup.
type ClientList struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Client []string `protobuf:"bytes,1,rep,name=client,proto3" json:"client,omitempty"`
}

func (x *ClientList) Reset() {
	*x = ClientList{}
	if protoimpl.UnsafeEnabled {
		mi := &file_client_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ClientList) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientList) ProtoMessage() {}

func (x *ClientList) ProtoReflect() protoreflect.Message {
	mi := &file_client_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClientList.ProtoReflect.Descriptor instead.
func (*ClientList) Descriptor() ([]byte, []int) {
	return file_client_proto_rawDescGZIP(), []int{4}
}

func (x *ClientList) GetClient() []string {
	if x != nil {
		return x.Client
	}
	return nil
}

var File_client_proto protoreflect.FileDescriptor

var file_client_proto_rawDesc = []byte{
	0x0a, 0x0c, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x09, 0x61, 0x70, 0x69, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x1a, 0x0a, 0x72, 0x6f, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x0a, 0x75, 0x73,
	0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x30, 0x0a, 0x07, 0x43, 0x6c, 0x69, 0x65,
	0x6e, 0x74, 0x73, 0x12, 0x25, 0x0a, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x18, 0x01, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x43, 0x6c, 0x69, 0x65,
	0x6e, 0x74, 0x52, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x22, 0xc3, 0x03, 0x0a, 0x06, 0x43,
	0x6c, 0x69, 0x65, 0x6e, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x75, 0x75, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x75, 0x75, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x14, 0x0a,
	0x05, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x6f, 0x77,
	0x6e, 0x65, 0x72, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69,
	0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73,
	0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12,
	0x1d, 0x0a, 0x0a, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x06, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x09, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x4b, 0x65, 0x79, 0x12, 0x1a,
	0x0a, 0x08, 0x76, 0x65, 0x72, 0x69, 0x66, 0x69, 0x65, 0x64, 0x18, 0x07, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x08, 0x76, 0x65, 0x72, 0x69, 0x66, 0x69, 0x65, 0x64, 0x12, 0x25, 0x0a, 0x0e, 0x61, 0x6c,
	0x6c, 0x6f, 0x77, 0x5f, 0x74, 0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x08, 0x20, 0x01,
	0x28, 0x08, 0x52, 0x0d, 0x61, 0x6c, 0x6c, 0x6f, 0x77, 0x54, 0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72,
	0x74, 0x12, 0x21, 0x0a, 0x0c, 0x61, 0x6c, 0x6c, 0x6f, 0x77, 0x5f, 0x69, 0x6e, 0x76, 0x69, 0x74,
	0x65, 0x18, 0x09, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0b, 0x61, 0x6c, 0x6c, 0x6f, 0x77, 0x49, 0x6e,
	0x76, 0x69, 0x74, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x6c, 0x6f, 0x67, 0x5f, 0x66, 0x69, 0x6c, 0x65,
	0x18, 0x0a, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6c, 0x6f, 0x67, 0x46, 0x69, 0x6c, 0x65, 0x12,
	0x2b, 0x0a, 0x08, 0x64, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65, 0x18, 0x0b, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x0f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x67, 0x61,
	0x74, 0x65, 0x52, 0x08, 0x64, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65, 0x12, 0x29, 0x0a, 0x08,
	0x61, 0x70, 0x69, 0x5f, 0x6b, 0x65, 0x79, 0x73, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x41, 0x70, 0x69, 0x4b, 0x65, 0x79, 0x73, 0x52, 0x07,
	0x61, 0x70, 0x69, 0x4b, 0x65, 0x79, 0x73, 0x12, 0x2b, 0x0a, 0x06, 0x61, 0x63, 0x63, 0x65, 0x73,
	0x73, 0x18, 0x0d, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e,
	0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x41, 0x63, 0x63, 0x65, 0x73, 0x73, 0x52, 0x06, 0x61, 0x63,
	0x63, 0x65, 0x73, 0x73, 0x12, 0x1a, 0x0a, 0x08, 0x64, 0x69, 0x73, 0x61, 0x62, 0x6c, 0x65, 0x64,
	0x18, 0x0e, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x64, 0x69, 0x73, 0x61, 0x62, 0x6c, 0x65, 0x64,
	0x22, 0x52, 0x0a, 0x0c, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x41, 0x63, 0x63, 0x65, 0x73, 0x73,
	0x12, 0x1f, 0x0a, 0x04, 0x75, 0x73, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x55, 0x73, 0x65, 0x72, 0x52, 0x04, 0x75, 0x73, 0x65,
	0x72, 0x12, 0x21, 0x0a, 0x05, 0x72, 0x6f, 0x6c, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x0b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x52, 0x6f, 0x6c, 0x65, 0x52, 0x05, 0x72,
	0x6f, 0x6c, 0x65, 0x73, 0x22, 0x64, 0x0a, 0x08, 0x44, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65,
	0x12, 0x1a, 0x0a, 0x08, 0x69, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x08, 0x69, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x12, 0x3c, 0x0a, 0x0b,
	0x72, 0x65, 0x73, 0x74, 0x72, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0e, 0x32, 0x1a, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x67, 0x61,
	0x74, 0x65, 0x52, 0x65, 0x73, 0x74, 0x72, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0b, 0x72,
	0x65, 0x73, 0x74, 0x72, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x24, 0x0a, 0x0a, 0x43, 0x6c,
	0x69, 0x65, 0x6e, 0x74, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x63, 0x6c, 0x69, 0x65,
	0x6e, 0x74, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74,
	0x2a, 0x7a, 0x0a, 0x13, 0x44, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65, 0x52, 0x65, 0x73, 0x74,
	0x72, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x1d, 0x0a, 0x19, 0x44, 0x45, 0x4c, 0x45, 0x47,
	0x41, 0x54, 0x45, 0x5f, 0x52, 0x45, 0x53, 0x54, 0x52, 0x49, 0x43, 0x54, 0x49, 0x4f, 0x4e, 0x5f,
	0x4e, 0x4f, 0x4e, 0x45, 0x10, 0x00, 0x12, 0x21, 0x0a, 0x1d, 0x44, 0x45, 0x4c, 0x45, 0x47, 0x41,
	0x54, 0x45, 0x5f, 0x52, 0x45, 0x53, 0x54, 0x52, 0x49, 0x43, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x56,
	0x49, 0x45, 0x57, 0x4f, 0x4e, 0x4c, 0x59, 0x10, 0x01, 0x12, 0x21, 0x0a, 0x1d, 0x44, 0x45, 0x4c,
	0x45, 0x47, 0x41, 0x54, 0x45, 0x5f, 0x52, 0x45, 0x53, 0x54, 0x52, 0x49, 0x43, 0x54, 0x49, 0x4f,
	0x4e, 0x5f, 0x43, 0x48, 0x41, 0x54, 0x4f, 0x4e, 0x4c, 0x59, 0x10, 0x02, 0x42, 0x29, 0x5a, 0x27,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x61, 0x63, 0x6b, 0x65,
	0x74, 0x66, 0x6c, 0x69, 0x6e, 0x67, 0x65, 0x72, 0x2f, 0x71, 0x32, 0x61, 0x64, 0x6d, 0x69, 0x6e,
	0x64, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_client_proto_rawDescOnce sync.Once
	file_client_proto_rawDescData = file_client_proto_rawDesc
)

func file_client_proto_rawDescGZIP() []byte {
	file_client_proto_rawDescOnce.Do(func() {
		file_client_proto_rawDescData = protoimpl.X.CompressGZIP(file_client_proto_rawDescData)
	})
	return file_client_proto_rawDescData
}

var file_client_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_client_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_client_proto_goTypes = []interface{}{
	(DelegateRestriction)(0), // 0: proto.DelegateRestriction
	(*Clients)(nil),          // 1: proto.Clients
	(*Client)(nil),           // 2: proto.Client
	(*ClientAccess)(nil),     // 3: proto.ClientAccess
	(*Delegate)(nil),         // 4: proto.Delegate
	(*ClientList)(nil),       // 5: proto.ClientList
	(*ApiKeys)(nil),          // 6: proto.ApiKeys
	(*User)(nil),             // 7: proto.User
	(*Role)(nil),             // 8: proto.Role
}
var file_client_proto_depIdxs = []int32{
	2, // 0: proto.Clients.client:type_name -> proto.Client
	4, // 1: proto.Client.delegate:type_name -> proto.Delegate
	6, // 2: proto.Client.api_keys:type_name -> proto.ApiKeys
	3, // 3: proto.Client.access:type_name -> proto.ClientAccess
	7, // 4: proto.ClientAccess.user:type_name -> proto.User
	8, // 5: proto.ClientAccess.roles:type_name -> proto.Role
	0, // 6: proto.Delegate.restriction:type_name -> proto.DelegateRestriction
	7, // [7:7] is the sub-list for method output_type
	7, // [7:7] is the sub-list for method input_type
	7, // [7:7] is the sub-list for extension type_name
	7, // [7:7] is the sub-list for extension extendee
	0, // [0:7] is the sub-list for field type_name
}

func init() { file_client_proto_init() }
func file_client_proto_init() {
	if File_client_proto != nil {
		return
	}
	file_api_proto_init()
	file_role_proto_init()
	file_user_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_client_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Clients); i {
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
		file_client_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Client); i {
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
		file_client_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ClientAccess); i {
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
		file_client_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Delegate); i {
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
		file_client_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ClientList); i {
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
			RawDescriptor: file_client_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_client_proto_goTypes,
		DependencyIndexes: file_client_proto_depIdxs,
		EnumInfos:         file_client_proto_enumTypes,
		MessageInfos:      file_client_proto_msgTypes,
	}.Build()
	File_client_proto = out.File
	file_client_proto_rawDesc = nil
	file_client_proto_goTypes = nil
	file_client_proto_depIdxs = nil
}

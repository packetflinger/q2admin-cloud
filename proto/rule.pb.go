// compile with:
// protoc --go_out=. --go_opt=paths=source_relative rule.proto

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v4.22.2
// source: rule.proto

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

type RuleType int32

const (
	RuleType_MUTE    RuleType = 0 // can't talk
	RuleType_BAN     RuleType = 1 // can't connect
	RuleType_MESSAGE RuleType = 2 // triggered message to user
	RuleType_STIFLE  RuleType = 3 // they can talk, but only once per amount of time
)

// Enum value maps for RuleType.
var (
	RuleType_name = map[int32]string{
		0: "MUTE",
		1: "BAN",
		2: "MESSAGE",
		3: "STIFLE",
	}
	RuleType_value = map[string]int32{
		"MUTE":    0,
		"BAN":     1,
		"MESSAGE": 2,
		"STIFLE":  3,
	}
)

func (x RuleType) Enum() *RuleType {
	p := new(RuleType)
	*p = x
	return p
}

func (x RuleType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (RuleType) Descriptor() protoreflect.EnumDescriptor {
	return file_rule_proto_enumTypes[0].Descriptor()
}

func (RuleType) Type() protoreflect.EnumType {
	return &file_rule_proto_enumTypes[0]
}

func (x RuleType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use RuleType.Descriptor instead.
func (RuleType) EnumDescriptor() ([]byte, []int) {
	return file_rule_proto_rawDescGZIP(), []int{0}
}

// A single key-value pair from a player's client.
type UserInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Property string `protobuf:"bytes,1,opt,name=property,proto3" json:"property,omitempty"`
	Value    string `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *UserInfo) Reset() {
	*x = UserInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rule_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UserInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UserInfo) ProtoMessage() {}

func (x *UserInfo) ProtoReflect() protoreflect.Message {
	mi := &file_rule_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UserInfo.ProtoReflect.Descriptor instead.
func (*UserInfo) Descriptor() ([]byte, []int) {
	return file_rule_proto_rawDescGZIP(), []int{0}
}

func (x *UserInfo) GetProperty() string {
	if x != nil {
		return x.Property
	}
	return ""
}

func (x *UserInfo) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

// For crafting rules based around specific times/timing.
//
// If a TimeSpec message is included in a rule, that rule only applies when
// the TimeSpec is true. The opposite is true if a TimeSpec is used in an
// exception.
//
// Times are relative to "midnight" [00:00] in the timezone the cloudadmin
// server is running, unless a timezone is included in the DATETIME_SPEC. So
// an "after 8am" equivelent rule on an America/Eastern [GMT-5] based admin
// server applied to a gameserver in the America/Pacific [GMT-8] zone will
// effectively result in a "after 5am" local rule.
//
// Daylight savings is not taken into account at all, that's up to the host
// machine.
//
// Don't use bad dates like February 29th when not in a leap year, bad shit
// will probably happen.
//
// DATETIME_SPEC examples
//   A string representation of a date/time. Only a few formats are accepted:
//     - "16:30:00" (hour:minute:second)
//     - "4:30PM"
//     - "2024-10-05" (year-month-day)
//     - "2024-10-05 16:30:00" (year-month-day hour:minute:second)
//
// INTERVAL_SPEC
//   It's a string representation of an amount of time. If no units are
//   included, the value will be assumed as seconds. Recognized units:
//     - "s" seconds
//     - "m" minutes
//     - "h" hours
//     - "d" days
//     - "w" weeks
//     - "M" months (note capital)
//     - "y" years
//   Examples
//     ["5m", "300s", "300", "0.083h", "5:00", "00:05:00"] = 5 minutes
//     ["3600", "3600s", "1h", "1:00:00", "01:00:00", "0.083d"] = 1 hour
//     ["21d", "3w", "0.057y", "0.7M"] = 3 weeks
//
// Before and after will be processeed when a player connects. After, every
// and play_time will also be checked during maintenance intervals, which runs
// on a schedule, so it's less granular. An "after 10:02AM" rule will match the
// next time maintenance runs, which is most likely every 5 minutes, so it rule
// won't be applied until 10:05.
type TimeSpec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Before   string `protobuf:"bytes,1,opt,name=before,proto3" json:"before,omitempty"`                     // DATETIME_SPEC
	After    string `protobuf:"bytes,2,opt,name=after,proto3" json:"after,omitempty"`                       // DATETIME_SPEC
	Every    string `protobuf:"bytes,3,opt,name=every,proto3" json:"every,omitempty"`                       // INTERVAL_SPEC
	PlayTime string `protobuf:"bytes,4,opt,name=play_time,json=playTime,proto3" json:"play_time,omitempty"` // INTERVAL_SPEC
}

func (x *TimeSpec) Reset() {
	*x = TimeSpec{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rule_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TimeSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TimeSpec) ProtoMessage() {}

func (x *TimeSpec) ProtoReflect() protoreflect.Message {
	mi := &file_rule_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TimeSpec.ProtoReflect.Descriptor instead.
func (*TimeSpec) Descriptor() ([]byte, []int) {
	return file_rule_proto_rawDescGZIP(), []int{1}
}

func (x *TimeSpec) GetBefore() string {
	if x != nil {
		return x.Before
	}
	return ""
}

func (x *TimeSpec) GetAfter() string {
	if x != nil {
		return x.After
	}
	return ""
}

func (x *TimeSpec) GetEvery() string {
	if x != nil {
		return x.Every
	}
	return ""
}

func (x *TimeSpec) GetPlayTime() string {
	if x != nil {
		return x.PlayTime
	}
	return ""
}

// If an exception matches as part of a rule that matches, the rule will
// be considered to have not matched.
//
// Instead of including password as an exception method, use the user_info. Players
// would need to include any passwords in their userinfo string anyway to get them
// to the server, so just look for a "password" or "pw" keyed ui value instead.
type Exception struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Description    []string    `protobuf:"bytes,7,rep,name=description,proto3" json:"description,omitempty"`                              // why? who is this exception for?
	Address        []string    `protobuf:"bytes,1,rep,name=address,proto3" json:"address,omitempty"`                                      // IP addr/cidr
	Hostname       []string    `protobuf:"bytes,6,rep,name=hostname,proto3" json:"hostname,omitempty"`                                    // PTR record (case-insensitive regex)
	Name           []string    `protobuf:"bytes,2,rep,name=name,proto3" json:"name,omitempty"`                                            // player name (case-insensitive regex)
	Client         []string    `protobuf:"bytes,3,rep,name=client,proto3" json:"client,omitempty"`                                        // game client name/version (case-sensitive regex)
	UserInfo       []*UserInfo `protobuf:"bytes,4,rep,name=user_info,json=userInfo,proto3" json:"user_info,omitempty"`                    // UI key/value pair
	ExpirationTime int64       `protobuf:"varint,5,opt,name=expiration_time,json=expirationTime,proto3" json:"expiration_time,omitempty"` // unix timestamp when exception no long valid
	Timespec       *TimeSpec   `protobuf:"bytes,8,opt,name=timespec,proto3" json:"timespec,omitempty"`                                    // time-related stuff
}

func (x *Exception) Reset() {
	*x = Exception{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rule_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Exception) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Exception) ProtoMessage() {}

func (x *Exception) ProtoReflect() protoreflect.Message {
	mi := &file_rule_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Exception.ProtoReflect.Descriptor instead.
func (*Exception) Descriptor() ([]byte, []int) {
	return file_rule_proto_rawDescGZIP(), []int{2}
}

func (x *Exception) GetDescription() []string {
	if x != nil {
		return x.Description
	}
	return nil
}

func (x *Exception) GetAddress() []string {
	if x != nil {
		return x.Address
	}
	return nil
}

func (x *Exception) GetHostname() []string {
	if x != nil {
		return x.Hostname
	}
	return nil
}

func (x *Exception) GetName() []string {
	if x != nil {
		return x.Name
	}
	return nil
}

func (x *Exception) GetClient() []string {
	if x != nil {
		return x.Client
	}
	return nil
}

func (x *Exception) GetUserInfo() []*UserInfo {
	if x != nil {
		return x.UserInfo
	}
	return nil
}

func (x *Exception) GetExpirationTime() int64 {
	if x != nil {
		return x.ExpirationTime
	}
	return 0
}

func (x *Exception) GetTimespec() *TimeSpec {
	if x != nil {
		return x.Timespec
	}
	return nil
}

// An player ACL. When a player connects to a cloudadmin-enabled gameserver, the
// server will attempt to match the player's information to each rule one at a time.
type Rule struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Uuid           string       `protobuf:"bytes,1,opt,name=uuid,proto3" json:"uuid,omitempty"`
	Type           RuleType     `protobuf:"varint,2,opt,name=type,proto3,enum=proto.RuleType" json:"type,omitempty"`
	Address        []string     `protobuf:"bytes,3,rep,name=address,proto3" json:"address,omitempty"`                                      // IP addr/cidr
	Hostname       []string     `protobuf:"bytes,18,rep,name=hostname,proto3" json:"hostname,omitempty"`                                   // PTR record (case-INsensitive regex)
	Name           []string     `protobuf:"bytes,4,rep,name=name,proto3" json:"name,omitempty"`                                            // player name (case-INsensitive regex)
	Client         []string     `protobuf:"bytes,5,rep,name=client,proto3" json:"client,omitempty"`                                        // game client/version (regex)
	UserInfo       []*UserInfo  `protobuf:"bytes,6,rep,name=user_info,json=userInfo,proto3" json:"user_info,omitempty"`                    // UI key/value pair (case-sensitive regex)
	Message        []string     `protobuf:"bytes,7,rep,name=message,proto3" json:"message,omitempty"`                                      // text to send on type MESSAGE or ban/mute message
	Vpn            bool         `protobuf:"varint,19,opt,name=vpn,proto3" json:"vpn,omitempty"`                                            // player IP is a VPN
	CreationTime   int64        `protobuf:"varint,8,opt,name=creation_time,json=creationTime,proto3" json:"creation_time,omitempty"`       // unix timestamp when rule was created
	ExpirationTime int64        `protobuf:"varint,9,opt,name=expiration_time,json=expirationTime,proto3" json:"expiration_time,omitempty"` // unix timestamp when no longer applies
	Delay          uint32       `protobuf:"varint,11,opt,name=delay,proto3" json:"delay,omitempty"`                                        // wait this man millisecs before action
	StifleLength   int32        `protobuf:"varint,20,opt,name=stifle_length,json=stifleLength,proto3" json:"stifle_length,omitempty"`      // seconds
	Description    []string     `protobuf:"bytes,14,rep,name=description,proto3" json:"description,omitempty"`                             // details on why this rule was created
	Exception      []*Exception `protobuf:"bytes,17,rep,name=exception,proto3" json:"exception,omitempty"`                                 // prevent a rule match
	Timespec       *TimeSpec    `protobuf:"bytes,21,opt,name=timespec,proto3" json:"timespec,omitempty"`                                   // time-related stuff
	Disabled       bool         `protobuf:"varint,22,opt,name=disabled,proto3" json:"disabled,omitempty"`                                  // ignore this rule?
}

func (x *Rule) Reset() {
	*x = Rule{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rule_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Rule) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Rule) ProtoMessage() {}

func (x *Rule) ProtoReflect() protoreflect.Message {
	mi := &file_rule_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Rule.ProtoReflect.Descriptor instead.
func (*Rule) Descriptor() ([]byte, []int) {
	return file_rule_proto_rawDescGZIP(), []int{3}
}

func (x *Rule) GetUuid() string {
	if x != nil {
		return x.Uuid
	}
	return ""
}

func (x *Rule) GetType() RuleType {
	if x != nil {
		return x.Type
	}
	return RuleType_MUTE
}

func (x *Rule) GetAddress() []string {
	if x != nil {
		return x.Address
	}
	return nil
}

func (x *Rule) GetHostname() []string {
	if x != nil {
		return x.Hostname
	}
	return nil
}

func (x *Rule) GetName() []string {
	if x != nil {
		return x.Name
	}
	return nil
}

func (x *Rule) GetClient() []string {
	if x != nil {
		return x.Client
	}
	return nil
}

func (x *Rule) GetUserInfo() []*UserInfo {
	if x != nil {
		return x.UserInfo
	}
	return nil
}

func (x *Rule) GetMessage() []string {
	if x != nil {
		return x.Message
	}
	return nil
}

func (x *Rule) GetVpn() bool {
	if x != nil {
		return x.Vpn
	}
	return false
}

func (x *Rule) GetCreationTime() int64 {
	if x != nil {
		return x.CreationTime
	}
	return 0
}

func (x *Rule) GetExpirationTime() int64 {
	if x != nil {
		return x.ExpirationTime
	}
	return 0
}

func (x *Rule) GetDelay() uint32 {
	if x != nil {
		return x.Delay
	}
	return 0
}

func (x *Rule) GetStifleLength() int32 {
	if x != nil {
		return x.StifleLength
	}
	return 0
}

func (x *Rule) GetDescription() []string {
	if x != nil {
		return x.Description
	}
	return nil
}

func (x *Rule) GetException() []*Exception {
	if x != nil {
		return x.Exception
	}
	return nil
}

func (x *Rule) GetTimespec() *TimeSpec {
	if x != nil {
		return x.Timespec
	}
	return nil
}

func (x *Rule) GetDisabled() bool {
	if x != nil {
		return x.Disabled
	}
	return false
}

// A collection of rules
type Rules struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Rule []*Rule `protobuf:"bytes,1,rep,name=rule,proto3" json:"rule,omitempty"`
}

func (x *Rules) Reset() {
	*x = Rules{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rule_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Rules) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Rules) ProtoMessage() {}

func (x *Rules) ProtoReflect() protoreflect.Message {
	mi := &file_rule_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Rules.ProtoReflect.Descriptor instead.
func (*Rules) Descriptor() ([]byte, []int) {
	return file_rule_proto_rawDescGZIP(), []int{4}
}

func (x *Rules) GetRule() []*Rule {
	if x != nil {
		return x.Rule
	}
	return nil
}

var File_rule_proto protoreflect.FileDescriptor

var file_rule_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x72, 0x75, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0x3c, 0x0a, 0x08, 0x55, 0x73, 0x65, 0x72, 0x49, 0x6e, 0x66, 0x6f, 0x12,
	0x1a, 0x0a, 0x08, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x22, 0x6b, 0x0a, 0x08, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x70, 0x65, 0x63, 0x12, 0x16, 0x0a,
	0x06, 0x62, 0x65, 0x66, 0x6f, 0x72, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x62,
	0x65, 0x66, 0x6f, 0x72, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x65,
	0x76, 0x65, 0x72, 0x79, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x65, 0x76, 0x65, 0x72,
	0x79, 0x12, 0x1b, 0x0a, 0x09, 0x70, 0x6c, 0x61, 0x79, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x70, 0x6c, 0x61, 0x79, 0x54, 0x69, 0x6d, 0x65, 0x22, 0x93,
	0x02, 0x0a, 0x09, 0x45, 0x78, 0x63, 0x65, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x20, 0x0a, 0x0b,
	0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x07, 0x20, 0x03, 0x28,
	0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x18,
	0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52,
	0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x1a, 0x0a, 0x08, 0x68, 0x6f, 0x73, 0x74,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x06, 0x20, 0x03, 0x28, 0x09, 0x52, 0x08, 0x68, 0x6f, 0x73, 0x74,
	0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x03,
	0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x63, 0x6c, 0x69, 0x65,
	0x6e, 0x74, 0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74,
	0x12, 0x2c, 0x0a, 0x09, 0x75, 0x73, 0x65, 0x72, 0x5f, 0x69, 0x6e, 0x66, 0x6f, 0x18, 0x04, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x55, 0x73, 0x65, 0x72,
	0x49, 0x6e, 0x66, 0x6f, 0x52, 0x08, 0x75, 0x73, 0x65, 0x72, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x27,
	0x0a, 0x0f, 0x65, 0x78, 0x70, 0x69, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x74, 0x69, 0x6d,
	0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0e, 0x65, 0x78, 0x70, 0x69, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x2b, 0x0a, 0x08, 0x74, 0x69, 0x6d, 0x65, 0x73,
	0x70, 0x65, 0x63, 0x18, 0x08, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x70, 0x65, 0x63, 0x52, 0x08, 0x74, 0x69, 0x6d, 0x65,
	0x73, 0x70, 0x65, 0x63, 0x22, 0x9f, 0x04, 0x0a, 0x04, 0x52, 0x75, 0x6c, 0x65, 0x12, 0x12, 0x0a,
	0x04, 0x75, 0x75, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x75, 0x75, 0x69,
	0x64, 0x12, 0x23, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32,
	0x0f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x52, 0x75, 0x6c, 0x65, 0x54, 0x79, 0x70, 0x65,
	0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73,
	0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73,
	0x12, 0x1a, 0x0a, 0x08, 0x68, 0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x12, 0x20, 0x03,
	0x28, 0x09, 0x52, 0x08, 0x68, 0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x03, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x12, 0x16, 0x0a, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x18, 0x05, 0x20, 0x03, 0x28, 0x09,
	0x52, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x12, 0x2c, 0x0a, 0x09, 0x75, 0x73, 0x65, 0x72,
	0x5f, 0x69, 0x6e, 0x66, 0x6f, 0x18, 0x06, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2e, 0x55, 0x73, 0x65, 0x72, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x08, 0x75, 0x73,
	0x65, 0x72, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x18, 0x07, 0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x12, 0x10, 0x0a, 0x03, 0x76, 0x70, 0x6e, 0x18, 0x13, 0x20, 0x01, 0x28, 0x08, 0x52, 0x03, 0x76,
	0x70, 0x6e, 0x12, 0x23, 0x0a, 0x0d, 0x63, 0x72, 0x65, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x74,
	0x69, 0x6d, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0c, 0x63, 0x72, 0x65, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x27, 0x0a, 0x0f, 0x65, 0x78, 0x70, 0x69, 0x72,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x0e, 0x65, 0x78, 0x70, 0x69, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x54, 0x69, 0x6d, 0x65,
	0x12, 0x14, 0x0a, 0x05, 0x64, 0x65, 0x6c, 0x61, 0x79, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x0d, 0x52,
	0x05, 0x64, 0x65, 0x6c, 0x61, 0x79, 0x12, 0x23, 0x0a, 0x0d, 0x73, 0x74, 0x69, 0x66, 0x6c, 0x65,
	0x5f, 0x6c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x18, 0x14, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0c, 0x73,
	0x74, 0x69, 0x66, 0x6c, 0x65, 0x4c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x12, 0x20, 0x0a, 0x0b, 0x64,
	0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x0e, 0x20, 0x03, 0x28, 0x09,
	0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x2e, 0x0a,
	0x09, 0x65, 0x78, 0x63, 0x65, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x11, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x10, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x45, 0x78, 0x63, 0x65, 0x70, 0x74, 0x69,
	0x6f, 0x6e, 0x52, 0x09, 0x65, 0x78, 0x63, 0x65, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x2b, 0x0a,
	0x08, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x70, 0x65, 0x63, 0x18, 0x15, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x0f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x70, 0x65, 0x63,
	0x52, 0x08, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x70, 0x65, 0x63, 0x12, 0x1a, 0x0a, 0x08, 0x64, 0x69,
	0x73, 0x61, 0x62, 0x6c, 0x65, 0x64, 0x18, 0x16, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x64, 0x69,
	0x73, 0x61, 0x62, 0x6c, 0x65, 0x64, 0x22, 0x28, 0x0a, 0x05, 0x52, 0x75, 0x6c, 0x65, 0x73, 0x12,
	0x1f, 0x0a, 0x04, 0x72, 0x75, 0x6c, 0x65, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0b, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x52, 0x75, 0x6c, 0x65, 0x52, 0x04, 0x72, 0x75, 0x6c, 0x65,
	0x2a, 0x36, 0x0a, 0x08, 0x52, 0x75, 0x6c, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12, 0x08, 0x0a, 0x04,
	0x4d, 0x55, 0x54, 0x45, 0x10, 0x00, 0x12, 0x07, 0x0a, 0x03, 0x42, 0x41, 0x4e, 0x10, 0x01, 0x12,
	0x0b, 0x0a, 0x07, 0x4d, 0x45, 0x53, 0x53, 0x41, 0x47, 0x45, 0x10, 0x02, 0x12, 0x0a, 0x0a, 0x06,
	0x53, 0x54, 0x49, 0x46, 0x4c, 0x45, 0x10, 0x03, 0x42, 0x29, 0x5a, 0x27, 0x67, 0x69, 0x74, 0x68,
	0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x61, 0x63, 0x6b, 0x65, 0x74, 0x66, 0x6c, 0x69,
	0x6e, 0x67, 0x65, 0x72, 0x2f, 0x71, 0x32, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x64, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_rule_proto_rawDescOnce sync.Once
	file_rule_proto_rawDescData = file_rule_proto_rawDesc
)

func file_rule_proto_rawDescGZIP() []byte {
	file_rule_proto_rawDescOnce.Do(func() {
		file_rule_proto_rawDescData = protoimpl.X.CompressGZIP(file_rule_proto_rawDescData)
	})
	return file_rule_proto_rawDescData
}

var file_rule_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_rule_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_rule_proto_goTypes = []interface{}{
	(RuleType)(0),     // 0: proto.RuleType
	(*UserInfo)(nil),  // 1: proto.UserInfo
	(*TimeSpec)(nil),  // 2: proto.TimeSpec
	(*Exception)(nil), // 3: proto.Exception
	(*Rule)(nil),      // 4: proto.Rule
	(*Rules)(nil),     // 5: proto.Rules
}
var file_rule_proto_depIdxs = []int32{
	1, // 0: proto.Exception.user_info:type_name -> proto.UserInfo
	2, // 1: proto.Exception.timespec:type_name -> proto.TimeSpec
	0, // 2: proto.Rule.type:type_name -> proto.RuleType
	1, // 3: proto.Rule.user_info:type_name -> proto.UserInfo
	3, // 4: proto.Rule.exception:type_name -> proto.Exception
	2, // 5: proto.Rule.timespec:type_name -> proto.TimeSpec
	4, // 6: proto.Rules.rule:type_name -> proto.Rule
	7, // [7:7] is the sub-list for method output_type
	7, // [7:7] is the sub-list for method input_type
	7, // [7:7] is the sub-list for extension type_name
	7, // [7:7] is the sub-list for extension extendee
	0, // [0:7] is the sub-list for field type_name
}

func init() { file_rule_proto_init() }
func file_rule_proto_init() {
	if File_rule_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_rule_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UserInfo); i {
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
		file_rule_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TimeSpec); i {
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
		file_rule_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Exception); i {
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
		file_rule_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Rule); i {
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
		file_rule_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Rules); i {
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
			RawDescriptor: file_rule_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_rule_proto_goTypes,
		DependencyIndexes: file_rule_proto_depIdxs,
		EnumInfos:         file_rule_proto_enumTypes,
		MessageInfos:      file_rule_proto_msgTypes,
	}.Build()
	File_rule_proto = out.File
	file_rule_proto_rawDesc = nil
	file_rule_proto_goTypes = nil
	file_rule_proto_depIdxs = nil
}

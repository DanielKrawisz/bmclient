// Code generated by protoc-gen-go.
// source: encoding.proto
// DO NOT EDIT!

/*
Package serialize is a generated protocol buffer package.

It is generated from these files:
	encoding.proto

It has these top-level messages:
	Entry
	Encoding
*/
package serialize

import proto "github.com/golang/protobuf/proto"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = math.Inf

// Entry is an entry in the database that contains a message and some related metadata.
type Entry struct {
	Sent             *bool     `protobuf:"varint,1,req" json:"Sent,omitempty"`
	AckReceived      *bool     `protobuf:"varint,2,req" json:"AckReceived,omitempty"`
	AckExpected      *bool     `protobuf:"varint,3,req" json:"AckExpected,omitempty"`
	DateReceived     *string   `protobuf:"bytes,4,req" json:"DateReceived,omitempty"`
	Flags            *int32    `protobuf:"varint,5,req" json:"Flags,omitempty"`
	Message          *Encoding `protobuf:"bytes,6,req" json:"Message,omitempty"`
	XXX_unrecognized []byte    `json:"-"`
}

func (m *Entry) Reset()         { *m = Entry{} }
func (m *Entry) String() string { return proto.CompactTextString(m) }
func (*Entry) ProtoMessage()    {}

func (m *Entry) GetSent() bool {
	if m != nil && m.Sent != nil {
		return *m.Sent
	}
	return false
}

func (m *Entry) GetAckReceived() bool {
	if m != nil && m.AckReceived != nil {
		return *m.AckReceived
	}
	return false
}

func (m *Entry) GetAckExpected() bool {
	if m != nil && m.AckExpected != nil {
		return *m.AckExpected
	}
	return false
}

func (m *Entry) GetDateReceived() string {
	if m != nil && m.DateReceived != nil {
		return *m.DateReceived
	}
	return ""
}

func (m *Entry) GetFlags() int32 {
	if m != nil && m.Flags != nil {
		return *m.Flags
	}
	return 0
}

func (m *Entry) GetMessage() *Encoding {
	if m != nil {
		return m.Message
	}
	return nil
}

// Encoding is a bitmessage encoded in format 2.
type Encoding struct {
	Encoding         *uint64 `protobuf:"varint,1,req" json:"Encoding,omitempty"`
	From             *string `protobuf:"bytes,2,req" json:"From,omitempty"`
	To               *string `protobuf:"bytes,3,req" json:"To,omitempty"`
	NonceTrials      *uint64 `protobuf:"varint,4,req" json:"NonceTrials,omitempty"`
	ExtraBytes       *uint64 `protobuf:"varint,5,req" json:"ExtraBytes,omitempty"`
	Behavior         *uint64 `protobuf:"varint,6,req" json:"Behavior,omitempty"`
	Subject          *string `protobuf:"bytes,7,opt" json:"Subject,omitempty"`
	Body             *string `protobuf:"bytes,8,opt" json:"Body,omitempty"`
	Ack              *string `protobuf:"bytes,9,opt" json:"Ack,omitempty"`
	Expiration       *string `protobuf:"bytes,10,opt" json:"Expiration,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *Encoding) Reset()         { *m = Encoding{} }
func (m *Encoding) String() string { return proto.CompactTextString(m) }
func (*Encoding) ProtoMessage()    {}

func (m *Encoding) GetEncoding() uint64 {
	if m != nil && m.Encoding != nil {
		return *m.Encoding
	}
	return 0
}

func (m *Encoding) GetFrom() string {
	if m != nil && m.From != nil {
		return *m.From
	}
	return ""
}

func (m *Encoding) GetTo() string {
	if m != nil && m.To != nil {
		return *m.To
	}
	return ""
}

func (m *Encoding) GetNonceTrials() uint64 {
	if m != nil && m.NonceTrials != nil {
		return *m.NonceTrials
	}
	return 0
}

func (m *Encoding) GetExtraBytes() uint64 {
	if m != nil && m.ExtraBytes != nil {
		return *m.ExtraBytes
	}
	return 0
}

func (m *Encoding) GetBehavior() uint64 {
	if m != nil && m.Behavior != nil {
		return *m.Behavior
	}
	return 0
}

func (m *Encoding) GetSubject() string {
	if m != nil && m.Subject != nil {
		return *m.Subject
	}
	return ""
}

func (m *Encoding) GetBody() string {
	if m != nil && m.Body != nil {
		return *m.Body
	}
	return ""
}

func (m *Encoding) GetAck() string {
	if m != nil && m.Ack != nil {
		return *m.Ack
	}
	return ""
}

func (m *Encoding) GetExpiration() string {
	if m != nil && m.Expiration != nil {
		return *m.Expiration
	}
	return ""
}
package brotatomodtypes

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"time"
)

// SerialType is a single byte indicating the "type" of the data that follows.
type SerialType uint8

const (
	// SerialTypeString represents any utf-8 encoded portion of data.
	// Immediately following this a uint32 indicating the length of the string should be provided.
	SerialTypeString SerialType = 0xdb

	SerialTypeInt8  SerialType = 0xd0
	SerialTypeInt16 SerialType = 0xd1
	SerialTypeInt32 SerialType = 0xd2
	SerialTypeInt64 SerialType = 0xd3

	SerialTypeFloat32 SerialType = 0xca
)

// MessageDictMappingHeader is a single byte for dict mapping start.
const MessageDictMappingHeader uint8 = 0xdf

// MessageType single byte indicating the "type" of the message.
type MessageType uint8

// String use to get Enum name.
func (mt MessageType) String() string {
	switch mt {
	case MessageTypeKeepAlive:
		return "KeepAlive"
	case MessageTypeTimeSeriesFull:
		return "TimeSeriesFull"
	case MessageTypeTimeSeriesDiff:
		return "TimeSeriesDiff"
	default:
		return "Unknown"
	}
}

// Valid use to check that MessageType is an enum.
func (mt MessageType) Valid() bool {
	switch mt {
	case MessageTypeKeepAlive, MessageTypeTimeSeriesFull, MessageTypeTimeSeriesDiff:
		return true
	default:
		return false
	}
}

const (
	// MessageTypeKeepAlive placeholder message type - can be used as keep-alive for TLS connection if necessary.
	MessageTypeKeepAlive MessageType = iota
	// MessageTypeTimeSeriesFull is a message type for events like wave/run start and end.
	MessageTypeTimeSeriesFull
	// MessageTypeTimeSeriesDiff message sent periodically throughout the run while in waves and shops. Contains only the changes since the last message.
	MessageTypeTimeSeriesDiff
)

// MessageReason single byte representing the game event that triggered the message.
type MessageReason uint8

const (
	MessageReasonNone MessageReason = iota
	MessageReasonShopEntered
	MessageReasonStartedWave
	MessageReasonRunEnded
	MessageReasonPoll
	MessageReasonConnect
)

// String use to get Enum name.
func (mr MessageReason) String() string {
	switch mr {
	case MessageReasonNone:
		return "None"
	case MessageReasonShopEntered:
		return "ShopEntered"
	case MessageReasonStartedWave:
		return "StartedWave"
	case MessageReasonRunEnded:
		return "RunEnded"
	case MessageReasonPoll:
		return "Poll"
	case MessageReasonConnect:
		return "Connect"
	default:
		return "Unknown"
	}
}

// MicroTime unix epoch time in microseconds.
type MicroTime int64

// MicroTimeFromTime construct MicroTime from time.Time.
func MicroTimeFromTime(t time.Time) MicroTime {
	return MicroTime(t.UnixNano() / 1000)
}

// Time
func (mt MicroTime) Time() time.Time {
	return time.Unix(0, int64(mt)*1000)
}

// String
func (mt MicroTime) String() string {
	return mt.Time().Format(time.RFC3339)
}

// MarshalJSON
func (mt MicroTime) MarshalJSON() ([]byte, error) {
	res := "\"" + mt.Time().Format(time.RFC3339) + "\""
	return []byte(res), nil
}

// format is:
// - message type (uint8)
// - message reason (uint8)
// - message timestamp (int64)
// - optional ExporterDictEncoder.encode_dict output (see for spec.)
type ExporterMessage struct {
	// MessageType is a single byte indicating the "type" of the message
	MessageType MessageType
	// MessageReason is a single byte representing the game event that triggered the message
	MessageReason MessageReason
	// MessageTimestamp is a unix epoch time in microseconds
	MessageTimestamp MicroTime

	// MessageBody is a reader for the message dictionary. Just reader providing an interface for reading key-values.
	// Iterator approach to avoid large map allocs - caller can still just build their own map without duplicating work.
	MessageBody DictReader
}

// DictKeyValue represents a mapping of a key to a value with a serial type.
type DictKeyValue struct {
	// Key short form of the original key brokered by the exporter. Uses 2 bytes instead of potentially 20+ bytes for data actually sent over the wire.
	Key uint16
	// MappedKey full original key. String assumed only to be valid until the next call to ReadNextKeyValue.
	MappedKey string
	// SerialType is a single byte indicating the "type" of the data that follows.
	SerialType SerialType
	// Value raw bytes of the value.
	Value []byte
}

// String use to parse the raw value bytes into their string representation. This is not an efficient method, used mostly for debug purposes.
func (dkv DictKeyValue) String() string {
	var val interface{}
	switch dkv.SerialType {
	case SerialTypeString:
		val = string(dkv.Value)
	case SerialTypeInt8:
		val = int8(dkv.Value[0])
	case SerialTypeInt16:
		val = binary.LittleEndian.Uint16(dkv.Value)
	case SerialTypeInt32:
		val = binary.LittleEndian.Uint32(dkv.Value)
	case SerialTypeInt64:
		val = binary.LittleEndian.Uint64(dkv.Value)
	case SerialTypeFloat32:
		val = math.Float32frombits(binary.LittleEndian.Uint32(dkv.Value))
	}

	return fmt.Sprintf("%v", val)
}

// AppendJSON appends the JSON representation of the value to the provided byte slice.
func (dkv DictKeyValue) AppendJSON(bts []byte) []byte {
	switch dkv.SerialType {
	case SerialTypeString:
		bts = append(bts, '"')
		bts = append(bts, dkv.Value...)
		bts = append(bts, '"')
	case SerialTypeInt8:
		bts = strconv.AppendInt(bts, int64(dkv.Value[0]), 10)
	case SerialTypeInt16:
		bts = strconv.AppendInt(bts, int64(binary.LittleEndian.Uint16(dkv.Value)), 10)
	case SerialTypeInt32:
		bts = strconv.AppendInt(bts, int64(binary.LittleEndian.Uint32(dkv.Value)), 10)
	case SerialTypeInt64:
		bts = strconv.AppendInt(bts, int64(binary.LittleEndian.Uint64(dkv.Value)), 10)
	case SerialTypeFloat32:
		bts = strconv.AppendFloat(bts, float64(math.Float32frombits(binary.LittleEndian.Uint32(dkv.Value))), 'f', -1, 32)
	}

	return bts
}

// DictReader interface for reading values from a message "body".
type DictReader interface {
	ReadNextKeyValue() (DictKeyValue, error)
	Size() int
}

// ModConfig represents a JSON mod config to be generated as part of creating the user.
type ModConfig struct {
	Enabled        bool                    `json:"enabled"`
	ConnectionData ModConfigConnectionData `json:"server_connection"`
}

// ModConfigConnectionData represents a JSON mod config connection data to be generated as part of creating the user.
type ModConfigConnectionData struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	HTTPS      bool   `json:"https"`
	VerifyHost bool   `json:"verify_host"`
	AuthToken  string `json:"auth_token"`
}

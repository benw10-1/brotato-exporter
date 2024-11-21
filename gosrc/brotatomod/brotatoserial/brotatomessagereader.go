package brotatoserial

import (
	"io"

	"github.com/benw10-1/brotato-exporter/brotatomod/brotatomodtypes"
	"github.com/benw10-1/brotato-exporter/errutil"
)

// dictMapping
type dictMapping struct {
	key uint16
	// value the translated JSON key value.
	value      string
	serialType brotatomodtypes.SerialType
}

// BrotatoMessageReader
type BrotatoMessageReader struct {
	serialReader *BrotatoSerialReader
	// convert key header
	dictMappingMap map[uint16]dictMapping
}

// NewMessageReader
func NewMessageReader(underlyingReader io.Reader, buf []byte) *BrotatoMessageReader {
	serialReader := NewSerialReader(underlyingReader, buf)

	return &BrotatoMessageReader{
		dictMappingMap: make(map[uint16]dictMapping),
		serialReader:   serialReader,
	}
}

// SetReader
func (mr *BrotatoMessageReader) SetReader(underlyingReader io.Reader) {
	mr.serialReader.SetReader(underlyingReader)
}

// ReadNextMessage
func (mr *BrotatoMessageReader) ReadNextMessage() (brotatomodtypes.ExporterMessage, error) {
	messageTypeByte, err := mr.serialReader.readUint8()
	if err != nil {
		return brotatomodtypes.ExporterMessage{}, errutil.NewStackError(err)
	}
	messageType := brotatomodtypes.MessageType(messageTypeByte)
	if !messageType.Valid() {
		return brotatomodtypes.ExporterMessage{}, errutil.NewStackErrorf("invalid message type %d", messageType)
	}

	messageReason, err := mr.serialReader.readUint8()
	if err != nil {
		return brotatomodtypes.ExporterMessage{}, errutil.NewStackError(err)
	}

	messageTimestamp, err := mr.serialReader.readInt64()
	if err != nil {
		return brotatomodtypes.ExporterMessage{}, errutil.NewStackError(err)
	}

	msg := brotatomodtypes.ExporterMessage{
		MessageType:      messageType,
		MessageReason:    brotatomodtypes.MessageReason(messageReason),
		MessageTimestamp: brotatomodtypes.MicroTime(messageTimestamp),
	}

	switch msg.MessageType {
	case brotatomodtypes.MessageTypeTimeSeriesFull, brotatomodtypes.MessageTypeTimeSeriesDiff:
		dr, err := NewDictReader(mr.serialReader, mr.dictMappingMap)
		if err != nil {
			return brotatomodtypes.ExporterMessage{}, errutil.NewStackError(err)
		}

		msg.MessageBody = dr
	default:
	}

	return msg, nil
}

// MappedKeyList
func (mr *BrotatoMessageReader) MappedKeyList() []string {
	keys := make([]string, 0, len(mr.dictMappingMap))
	for _, v := range mr.dictMappingMap {
		keys = append(keys, v.value)
	}

	return keys
}

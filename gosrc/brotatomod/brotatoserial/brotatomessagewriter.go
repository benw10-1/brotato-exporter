package brotatoserial

import (
	"errors"
	"io"

	"github.com/benw10-1/brotato-exporter/brotatomod/brotatomodtypes"
	"github.com/benw10-1/brotato-exporter/errutil"
)

// BrotatoMessageWriter
type BrotatoMessageWriter struct {
	serialWriter *BrotatoSerialWriter
	dictWriter   *BrotatoDictWriter
}

// NewMessageWriter
func NewMessageWriter(serialWriter *BrotatoSerialWriter) *BrotatoMessageWriter {
	return &BrotatoMessageWriter{
		serialWriter: serialWriter,
		dictWriter:   NewDictWriter(serialWriter),
	}
}

// WriteMessage
func (bmw *BrotatoMessageWriter) WriteMessage(msg *brotatomodtypes.ExporterMessage) error {
	err := bmw.serialWriter.writeUint8(uint8(msg.MessageType))
	if err != nil {
		return errutil.NewStackError(err)
	}

	err = bmw.serialWriter.writeUint8(uint8(msg.MessageReason))
	if err != nil {
		return errutil.NewStackError(err)
	}

	err = bmw.serialWriter.writeUint64(uint64(msg.MessageTimestamp))
	if err != nil {
		return errutil.NewStackError(err)
	}

	if msg.MessageBody == nil {
		return nil
	}

	err = bmw.dictWriter.EncodeDict(msg.MessageBody)
	if err != nil && !errors.Is(err, io.EOF) {
		return errutil.NewStackError(err)
	}

	return nil
}

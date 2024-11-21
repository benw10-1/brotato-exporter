package brotatoserial

import (
	"encoding/binary"
	"io"

	"github.com/benw10-1/brotato-exporter/errutil"
)

// BrotatoSerialWriter binary reader. Use to consume concrete types from a given reader.
type BrotatoSerialWriter struct {
	// underlyingWriter any implementation of io.Reader. Caller should close this to stop the reader. All errors including io.EOF are passed up.
	underlyingWriter io.Writer
	valBuf           []byte
}

// NewSerialWriter constructor for BrotatoSerialWriter.
func NewSerialWriter(underlyingWriter io.Writer) *BrotatoSerialWriter {
	return &BrotatoSerialWriter{
		underlyingWriter: underlyingWriter,
		valBuf:           make([]byte, 8),
	}
}

// writeUint8
func (sr *BrotatoSerialWriter) writeUint8(v uint8) error {
	_, err := sr.underlyingWriter.Write([]byte{v})
	if err != nil {
		return errutil.NewStackError(err)
	}

	return nil
}

// writeUint16
func (sr *BrotatoSerialWriter) writeUint16(v uint16) error {
	sr.valBuf = binary.LittleEndian.AppendUint16(sr.valBuf[:0], v)

	_, err := sr.underlyingWriter.Write(sr.valBuf)
	if err != nil {
		return errutil.NewStackError(err)
	}

	return nil
}

// writeUint32
func (sr *BrotatoSerialWriter) writeUint32(v uint32) error {
	sr.valBuf = binary.LittleEndian.AppendUint32(sr.valBuf[:0], v)

	_, err := sr.underlyingWriter.Write(sr.valBuf)
	if err != nil {
		return errutil.NewStackError(err)
	}

	return nil
}

// writeInt64
func (sr *BrotatoSerialWriter) writeUint64(v uint64) error {
	sr.valBuf = binary.LittleEndian.AppendUint64(sr.valBuf[:0], v)

	_, err := sr.underlyingWriter.Write(sr.valBuf)
	if err != nil {
		return errutil.NewStackError(err)
	}

	return nil
}

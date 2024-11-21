package brotatoserial

import (
	"encoding/binary"
	"io"

	"github.com/benw10-1/brotato-exporter/errutil"
)

// BrotatoSerialReader binary reader. Use to consume concrete types from a given reader.
type BrotatoSerialReader struct {
	// underlyingReader any implementation of io.Reader. Caller should close this to stop the reader. All errors including io.EOF are passed up.
	underlyingReader io.Reader
	// msgBuf scratch bytes for reading from the underlying reader. Bytes returned by reads are only valid until the next call to read
	msgBuf []byte

	// peekedByte single unconsumed byte. Useful for checking header and type values.
	peekedByte bool
}

// NewSerialReader constructor for BrotatoSerialReader.
func NewSerialReader(underlyingReader io.Reader, buf []byte) *BrotatoSerialReader {
	if buf == nil {
		buf = make([]byte, 1024)
	}
	return &BrotatoSerialReader{
		underlyingReader: underlyingReader,
		msgBuf:           buf,
	}
}

// SetReader
func (sr *BrotatoSerialReader) SetReader(underlyingReader io.Reader) {
	sr.underlyingReader = underlyingReader
	sr.peekedByte = false
}

// requires
func requires(buf []byte, length int) []byte {
	if len(buf) < length {
		incrementCount := length / 1024
		buf = make([]byte, incrementCount+2*1024)
	}

	return buf
}

// readBytes
func (sr *BrotatoSerialReader) readBytes(count int) ([]byte, error) {
	if sr.underlyingReader == nil {
		return nil, errutil.NewStackError("underlying reader is nil")
	}

	if count <= 0 {
		return nil, errutil.NewStackError("reading no bytes")
	}

	sr.msgBuf = requires(sr.msgBuf, count)

	var startIdx int
	if sr.peekedByte {
		sr.peekedByte = false
		if count == 1 {
			return sr.msgBuf[:1], nil
		}

		startIdx = 1
	}

	n, err := sr.underlyingReader.Read(sr.msgBuf[startIdx:count])
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	if n != count {
		return nil, errutil.NewStackError(io.ErrUnexpectedEOF)
	}

	return sr.msgBuf[:count], nil
}

// peekUint8
func (sr *BrotatoSerialReader) peekUint8() (uint8, error) {
	if sr.peekedByte {
		return sr.msgBuf[0], nil
	}

	bytes, err := sr.readBytes(1)
	if err != nil {
		return 0, errutil.NewStackError(err)
	}

	sr.peekedByte = true
	return bytes[0], nil
}

// readUint8
func (sr *BrotatoSerialReader) readUint8() (uint8, error) {
	bytes, err := sr.readBytes(1)
	if err != nil {
		return 0, errutil.NewStackError(err)
	}

	return bytes[0], nil
}

// readUint16
func (sr *BrotatoSerialReader) readUint16() (uint16, error) {
	bytes, err := sr.readBytes(2)
	if err != nil {
		return 0, errutil.NewStackError(err)
	}

	return binary.LittleEndian.Uint16(bytes), nil
}

// readUint32
func (sr *BrotatoSerialReader) readUint32() (uint32, error) {
	bytes, err := sr.readBytes(4)
	if err != nil {
		return 0, errutil.NewStackError(err)
	}

	return binary.LittleEndian.Uint32(bytes), nil
}

// readInt64
func (sr *BrotatoSerialReader) readInt64() (int64, error) {
	bytes, err := sr.readBytes(8)
	if err != nil {
		return 0, errutil.NewStackError(err)
	}

	return int64(binary.LittleEndian.Uint64(bytes)), nil
}

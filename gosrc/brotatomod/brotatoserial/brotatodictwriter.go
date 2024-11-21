package brotatoserial

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/benw10-1/brotato-exporter/brotatomod/brotatomodtypes"
	"github.com/benw10-1/brotato-exporter/errutil"
)

// BrotatoDictWriter
type BrotatoDictWriter struct {
	serialWriter *BrotatoSerialWriter
	keyMappings  map[string]brotatomodtypes.DictKeyValue
}

// NewDictWriter
func NewDictWriter(serialWriter *BrotatoSerialWriter) *BrotatoDictWriter {
	return &BrotatoDictWriter{
		serialWriter: serialWriter,
		keyMappings:  make(map[string]brotatomodtypes.DictKeyValue),
	}
}

// EncodeDict
func (dw *BrotatoDictWriter) EncodeDict(dict brotatomodtypes.DictReader) error {
	headerBuf := make([]byte, 0, 1024)
	headerBuf = append(headerBuf, brotatomodtypes.MessageDictMappingHeader, 0, 0)

	bodyBuf := make([]byte, 0, 1024)
	bodyBuf = append(bodyBuf, 0, 0)

	kvIdx := len(dw.keyMappings)
	newCount := 0
	for {
		kv, err := dict.ReadNextKeyValue()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return errutil.NewStackError(err)
		}

		mapped, ok := dw.keyMappings[kv.MappedKey]
		if ok {
			bodyBuf = binary.LittleEndian.AppendUint16(bodyBuf, mapped.Key)
			bodyBuf = append(bodyBuf, mapped.Value...)
			continue
		}

		mapped = brotatomodtypes.DictKeyValue{
			Key:        uint16(kvIdx),
			MappedKey:  kv.MappedKey,
			SerialType: kv.SerialType,
			Value:      kv.Value,
		}
		dw.keyMappings[kv.MappedKey] = mapped
		kvIdx++
		newCount++

		headerBuf = binary.LittleEndian.AppendUint16(headerBuf, mapped.Key)
		headerBuf = append(headerBuf, byte(mapped.SerialType))
		headerBuf = binary.LittleEndian.AppendUint16(headerBuf, uint16(len(mapped.MappedKey)))
		headerBuf = append(headerBuf, []byte(mapped.MappedKey)...)
	}

	binary.LittleEndian.PutUint16(headerBuf[1:], uint16(newCount))
	binary.LittleEndian.PutUint16(bodyBuf, uint16(dict.Size()))

	_, err := dw.serialWriter.underlyingWriter.Write(headerBuf)
	if err != nil {
		return errutil.NewStackError(err)
	}

	_, err = dw.serialWriter.underlyingWriter.Write(bodyBuf)
	if err != nil {
		return errutil.NewStackError(err)
	}

	return nil
}

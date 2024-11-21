package brotatoserial

import (
	"errors"
	"fmt"
	"io"

	"github.com/benw10-1/brotato-exporter/brotatomod/brotatomodtypes"
	"github.com/benw10-1/brotato-exporter/errutil"
)

// BrotatoDictReader implements the DictReader interface.
type BrotatoDictReader struct {
	// serialReader
	serialReader *BrotatoSerialReader

	closed   bool
	keyCount uint16

	readCount uint16

	dictMappingMap map[uint16]dictMapping
}

// NewDictReader
func NewDictReader(serialReader *BrotatoSerialReader, dictMappingMap map[uint16]dictMapping) (*BrotatoDictReader, error) {
	header, err := serialReader.peekUint8()
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	if header != brotatomodtypes.MessageDictMappingHeader {
		return nil, errutil.NewStackError(errors.New("invalid dict mapping header"))
	}

	// actually consume the byte
	_, err = serialReader.readUint8()
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	dr := &BrotatoDictReader{
		serialReader:   serialReader,
		dictMappingMap: dictMappingMap,
	}

	err = dr.readDictHeader()
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	return dr, nil
}

// readDictHeader if expecting a dict type, check that the next byte
func (dr *BrotatoDictReader) readDictHeader() error {
	err := dr.readMessageDictMapping()
	if err != nil {
		return errutil.NewStackError(err)
	}

	dr.keyCount, err = dr.serialReader.readUint16()
	if err != nil {
		return errutil.NewStackError(err)
	}

	return nil
}

// iface implementation check
var _ brotatomodtypes.DictReader = &BrotatoDictReader{}

var zeroDictKeyVal = brotatomodtypes.DictKeyValue{}

// ReadNextKeyValue reads the next key-value pair from the message.
// Value bytes are only valid until the next call to ReadNextKeyValue.
// Returns EOF when there are no more key-value pairs.
// Format of the dict:
// - MESSAGE_DICT_MAPPING_HEADER (uint8)
// - new key count (uint16)
// - (for each new key)
// -- key mapping (uint16)
// -- SERIAL_TYPE (uint8)
// -- key string length (uint16)
// -- key string (variable [uint8, ...])
// - amount of key-value pairs (uint16)
// - (for each key in dict)
// -- $key_mappings[key] (uint16)
// -- value bytes (variable - either uintx or [uint32 {length}, uint8, uint8, ...]
func (dr *BrotatoDictReader) ReadNextKeyValue() (brotatomodtypes.DictKeyValue, error) {
	if dr.closed {
		return zeroDictKeyVal, errutil.NewStackError("reader is closed")
	}

	if dr.keyCount == 0 {
		dr.closed = true
		return zeroDictKeyVal, errutil.NewStackError(io.EOF)
	}

	if dr.readCount >= dr.keyCount {
		dr.closed = true
		return zeroDictKeyVal, errutil.NewStackError(io.EOF)
	}

	// -- key string length (uint16)
	key, err := dr.serialReader.readUint16()
	if err != nil {
		return zeroDictKeyVal, errutil.NewStackError(err)
	}

	mappedVal, ok := dr.dictMappingMap[key]
	if !ok {
		return zeroDictKeyVal, errutil.NewStackError(fmt.Sprintf("key not found in dict mapping - %d", key))
	}

	var valueBytes []byte
	switch mappedVal.serialType {
	case brotatomodtypes.SerialTypeString:
		length, err := dr.serialReader.readUint32()
		if err != nil {
			return zeroDictKeyVal, errutil.NewStackError(err)
		}

		valueBytes, err = dr.serialReader.readBytes(int(length))
		if err != nil {
			return zeroDictKeyVal, errutil.NewStackError(err)
		}
	case brotatomodtypes.SerialTypeInt8:
		valueBytes, err = dr.serialReader.readBytes(1)
		if err != nil {
			return zeroDictKeyVal, errutil.NewStackError(err)
		}
	case brotatomodtypes.SerialTypeInt16:
		valueBytes, err = dr.serialReader.readBytes(2)
		if err != nil {
			return zeroDictKeyVal, errutil.NewStackError(err)
		}
	case brotatomodtypes.SerialTypeInt32:
		valueBytes, err = dr.serialReader.readBytes(4)
		if err != nil {
			return zeroDictKeyVal, errutil.NewStackError(err)
		}
	case brotatomodtypes.SerialTypeInt64:
		valueBytes, err = dr.serialReader.readBytes(8)
		if err != nil {
			return zeroDictKeyVal, errutil.NewStackError(err)
		}
	case brotatomodtypes.SerialTypeFloat32:
		valueBytes, err = dr.serialReader.readBytes(4)
		if err != nil {
			return zeroDictKeyVal, errutil.NewStackError(err)
		}
	default:
		return zeroDictKeyVal, errors.New("unknown serial type")
	}

	dr.readCount++

	return brotatomodtypes.DictKeyValue{
		Key:        key,
		SerialType: mappedVal.serialType,
		MappedKey:  mappedVal.value,
		Value:      valueBytes,
	}, nil
}

// Size returns the number of key-value pairs in the dict.
func (dr *BrotatoDictReader) Size() int {
	return int(dr.keyCount)
}

// readMessageDictMapping read dict mapping header from msg.
//
// Format of the dict mapping header:
// - new key count (uint16)
// - (for each new key)
// -- key mapping (uint16)
// -- SERIAL_TYPE (uint8)
// -- key string length (uint16)
// -- key string (variable [uint8, ...])
func (dr *BrotatoDictReader) readMessageDictMapping() error {
	newKeyCount, err := dr.serialReader.readUint16()
	if err != nil {
		return errutil.NewStackError(err)
	}

	for i := 0; i < int(newKeyCount); i++ {
		keyMapping, err := dr.serialReader.readUint16()
		if err != nil {
			return errutil.NewStackError(err)
		}

		serialType, err := dr.serialReader.readUint8()
		if err != nil {
			return errutil.NewStackError(err)
		}

		keyStrLength, err := dr.serialReader.readUint16()
		if err != nil {
			return errutil.NewStackError(err)
		}

		keyStrBytes, err := dr.serialReader.readBytes(int(keyStrLength))
		if err != nil {
			return errutil.NewStackError(err)
		}

		dr.dictMappingMap[keyMapping] = dictMapping{
			key:        keyMapping,
			value:      string(keyStrBytes),
			serialType: brotatomodtypes.SerialType(serialType),
		}
	}

	return nil
}

// MapDictReader
type MapDictReader struct {
	dict    map[string]brotatomodtypes.DictKeyValue
	keyList []string
	curIdx  int
}

// NewMapDictReader
func NewMapDictReader(dict map[string]brotatomodtypes.DictKeyValue) *MapDictReader {
	keyList := make([]string, 0, len(dict))
	for k := range dict {
		keyList = append(keyList, k)
	}

	return &MapDictReader{
		dict:    dict,
		keyList: keyList,
	}
}

// ReadNextKeyValue
func (mdr *MapDictReader) ReadNextKeyValue() (brotatomodtypes.DictKeyValue, error) {
	if mdr.curIdx >= len(mdr.keyList) {
		return brotatomodtypes.DictKeyValue{}, io.EOF
	}

	key := mdr.keyList[mdr.curIdx]
	mdr.curIdx++

	return mdr.dict[key], nil
}

// Size
func (mdr *MapDictReader) Size() int {
	return len(mdr.keyList)
}

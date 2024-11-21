package brotatoserial

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/benw10-1/brotato-exporter/brotatomod/brotatomodtypes"
	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/stretchr/testify/require"
)

func TestDictWriter(t *testing.T) {

	type testCase struct {
		name string
		dict map[string]interface{}
	}

	tcs := []testCase{
		{
			name: "strings",
			dict: map[string]interface{}{
				"key":  "value",
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
		{
			name: "numbers",
			dict: map[string]interface{}{
				"key":  1,
				"key1": 2,
				"key2": float32(3.24),
				"key3": 4,
			},
		},
		{
			name: "mixed",
			dict: map[string]interface{}{
				"key":  "value",
				"key1": 1,
				"key2": 2,
				"key3": float32(3.77),
				"key4": "value4",
				"key5": 5,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			asserter := require.New(t)

			kvMap := make(map[string]brotatomodtypes.DictKeyValue)

			for k, v := range tc.dict {
				kv, err := newKeyValue(k, v)
				asserter.NoError(err)

				kvMap[k] = kv
			}

			w := bytes.NewBuffer(nil)

			serialW := NewSerialWriter(w)

			dw := NewDictWriter(serialW)
			dr := NewMapDictReader(kvMap)

			err := dw.EncodeDict(dr)
			asserter.NoError(err)

			resSerialReader := NewSerialReader(w, make([]byte, 0, 1024))

			resDr, err := NewDictReader(resSerialReader, map[uint16]dictMapping{})
			asserter.NoError(err)

			for {
				kv, err := resDr.ReadNextKeyValue()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}

					asserter.NoError(err)
				}

				expectedKV, ok := kvMap[kv.MappedKey]
				asserter.True(ok)

				asserter.Equal(expectedKV.SerialType, kv.SerialType)
				asserter.Equal(expectedKV.Value, kv.Value)
			}
		})
	}
}

func newKeyValue(key string, value interface{}) (brotatomodtypes.DictKeyValue, error) {
	kv := brotatomodtypes.DictKeyValue{
		MappedKey: key,
	}
	switch v := value.(type) {
	case string:
		kv.SerialType = brotatomodtypes.SerialTypeString
		kv.Value = []byte(v)
		if len(kv.Value) > (1 << 16) {
			return kv, errutil.NewStackErrorf("string value too long: %d", len(kv.Value))
		}
	case []byte:
		kv.SerialType = brotatomodtypes.SerialTypeString
		kv.Value = v
		if len(kv.Value) > (1 << 16) {
			return kv, errutil.NewStackErrorf("string value too long: %d", len(kv.Value))
		}
	case int:
		kv.SerialType = brotatomodtypes.SerialTypeInt64
		kv.Value = binary.LittleEndian.AppendUint64(nil, uint64(v))
	case uint:
		kv.SerialType = brotatomodtypes.SerialTypeInt64
		kv.Value = binary.LittleEndian.AppendUint64(nil, uint64(v))
	case int64:
		kv.SerialType = brotatomodtypes.SerialTypeInt64
		kv.Value = binary.LittleEndian.AppendUint64(nil, uint64(v))
	case int32:
		kv.SerialType = brotatomodtypes.SerialTypeInt32
		kv.Value = binary.LittleEndian.AppendUint32(nil, uint32(v))
	case int16:
		kv.SerialType = brotatomodtypes.SerialTypeInt16
		kv.Value = binary.LittleEndian.AppendUint16(nil, uint16(v))
	case int8:
		kv.SerialType = brotatomodtypes.SerialTypeInt8
		kv.Value = []byte{byte(v)}
	case byte:
		kv.SerialType = brotatomodtypes.SerialTypeInt8
		kv.Value = []byte{v}
	case float32:
		kv.SerialType = brotatomodtypes.SerialTypeFloat32
		kv.Value = binary.LittleEndian.AppendUint32(nil, math.Float32bits(v))

	default:
		return kv, errutil.NewStackErrorf("unsupported value type %T", value)
	}

	return kv, nil
}

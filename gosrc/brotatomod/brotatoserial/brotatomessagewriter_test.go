package brotatoserial

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/benw10-1/brotato-exporter/brotatomod/brotatomodtypes"
	"github.com/stretchr/testify/require"
)

func TestMessageWriter(t *testing.T) {
	type testCase struct {
		name    string
		msg     *brotatomodtypes.ExporterMessage
		msgBody map[string]interface{}
	}

	nowTimestamp := brotatomodtypes.MicroTimeFromTime(time.Now())

	tcs := []testCase{
		{
			name: "keep-alive",
			msg: &brotatomodtypes.ExporterMessage{
				MessageType:      brotatomodtypes.MessageTypeKeepAlive,
				MessageReason:    brotatomodtypes.MessageReasonPoll,
				MessageTimestamp: nowTimestamp,
			},
		},
		{
			name: "full timeseries",
			msg: &brotatomodtypes.ExporterMessage{
				MessageType:      brotatomodtypes.MessageTypeTimeSeriesFull,
				MessageReason:    brotatomodtypes.MessageReasonShopEntered,
				MessageTimestamp: nowTimestamp,
			},
			msgBody: map[string]interface{}{
				"chal_recycling_current":         0,
				"consumables_picked_up_this_run": 2,
				"current_character":              "character_crazy",
				"current_health":                 11,
				"current_level":                  1,
				"current_xp":                     float32(10),
			},
		},
		{
			name: "diff timeseries",
			msg: &brotatomodtypes.ExporterMessage{
				MessageType:      brotatomodtypes.MessageTypeTimeSeriesFull,
				MessageReason:    brotatomodtypes.MessageReasonShopEntered,
				MessageTimestamp: nowTimestamp,
			},
			msgBody: map[string]interface{}{
				"current_character": "-",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			asserter := require.New(t)

			kvMap := make(map[string]brotatomodtypes.DictKeyValue)

			for k, v := range tc.msgBody {
				kv, err := newKeyValue(k, v)
				asserter.NoError(err)

				kvMap[k] = kv
			}

			dictReader := NewMapDictReader(kvMap)

			w := bytes.NewBuffer(nil)

			serialW := NewSerialWriter(w)

			mw := NewMessageWriter(serialW)

			tc.msg.MessageBody = dictReader

			err := mw.WriteMessage(tc.msg)
			asserter.NoError(err)

			messageResReader := NewMessageReader(w, make([]byte, 0, 1024))
			resMsg, err := messageResReader.ReadNextMessage()
			asserter.NoError(err)

			asserter.Equal(tc.msg.MessageType, resMsg.MessageType)
			asserter.Equal(tc.msg.MessageReason, resMsg.MessageReason)
			asserter.Equal(tc.msg.MessageTimestamp, resMsg.MessageTimestamp)

			if tc.msgBody == nil {
				return
			}

			asserter.NotNil(resMsg.MessageBody)

			asserter.Equal(len(kvMap), resMsg.MessageBody.Size())

			for {
				kv, err := resMsg.MessageBody.ReadNextKeyValue()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}

					asserter.NoError(err)
				}

				asserter.Contains(kvMap, kv.MappedKey)
				asserter.Equal(kvMap[kv.MappedKey].SerialType, kv.SerialType)
				asserter.Equal(kvMap[kv.MappedKey].Value, kv.Value)
			}
		})
	}

	// testing a bunch of messages in a single reader one after the other
	t.Run("game-like", func(t *testing.T) {
		asserter := require.New(t)

		msgList := []testCase{
			{
				msg: &brotatomodtypes.ExporterMessage{
					MessageType:      brotatomodtypes.MessageTypeKeepAlive,
					MessageReason:    brotatomodtypes.MessageReasonPoll,
					MessageTimestamp: nowTimestamp,
				},
			},
			{
				msg: &brotatomodtypes.ExporterMessage{
					MessageType:      brotatomodtypes.MessageTypeTimeSeriesFull,
					MessageReason:    brotatomodtypes.MessageReasonShopEntered,
					MessageTimestamp: nowTimestamp,
				},
				msgBody: map[string]interface{}{
					"chal_recycling_current":         0,
					"consumables_picked_up_this_run": 2,
					"current_character":              "character_crazy",
					"current_health":                 11,
					"current_level":                  1,
					"current_xp":                     float32(10),
				},
			},
			{
				msg: &brotatomodtypes.ExporterMessage{
					MessageType:      brotatomodtypes.MessageTypeTimeSeriesFull,
					MessageReason:    brotatomodtypes.MessageReasonStartedWave,
					MessageTimestamp: nowTimestamp,
				},
				msgBody: map[string]interface{}{
					"chal_recycling_current":         5,
					"consumables_picked_up_this_run": 3,
					"current_character":              "character_crazy",
					"current_health":                 19,
					"current_level":                  1,
					"current_xp":                     float32(16.4),
				},
			},
			{
				msg: &brotatomodtypes.ExporterMessage{
					MessageType:      brotatomodtypes.MessageTypeTimeSeriesDiff,
					MessageReason:    brotatomodtypes.MessageReasonPoll,
					MessageTimestamp: nowTimestamp,
				},
				msgBody: map[string]interface{}{
					"chal_recycling_current": 7,
					"current_level":          2,
					"current_xp":             float32(32.7),
				},
			},
			{
				msg: &brotatomodtypes.ExporterMessage{
					MessageType:      brotatomodtypes.MessageTypeTimeSeriesDiff,
					MessageReason:    brotatomodtypes.MessageReasonPoll,
					MessageTimestamp: nowTimestamp,
				},
				msgBody: map[string]interface{}{
					"chal_recycling_current": 7,
					"current_level":          2,
					"current_xp":             float32(45.1),
					"effects_stat_dodge":     -36,
				},
			},
			{
				msg: &brotatomodtypes.ExporterMessage{
					MessageType:      brotatomodtypes.MessageTypeKeepAlive,
					MessageReason:    brotatomodtypes.MessageReasonPoll,
					MessageTimestamp: nowTimestamp,
				},
			},
		}

		w := bytes.NewBuffer(nil)

		serialW := NewSerialWriter(w)

		mw := NewMessageWriter(serialW)

		messageKVs := make([]map[string]brotatomodtypes.DictKeyValue, len(msgList))

		for i, msg := range msgList {
			kvMap := make(map[string]brotatomodtypes.DictKeyValue)

			for k, v := range msg.msgBody {
				kv, err := newKeyValue(k, v)
				asserter.NoError(err)

				kvMap[k] = kv
			}

			messageKVs[i] = kvMap

			msg.msg.MessageBody = NewMapDictReader(kvMap)

			err := mw.WriteMessage(msg.msg)
			asserter.NoError(err)
		}

		messageReader := NewMessageReader(w, make([]byte, 0, 1024))

		for i, msg := range msgList {
			resMsg, err := messageReader.ReadNextMessage()
			asserter.NoError(err)

			asserter.Equal(msg.msg.MessageType, resMsg.MessageType)
			asserter.Equal(msg.msg.MessageReason, resMsg.MessageReason)
			asserter.Equal(msg.msg.MessageTimestamp, resMsg.MessageTimestamp)

			if msg.msgBody == nil {
				continue
			}

			asserter.Equal(len(messageKVs[i]), resMsg.MessageBody.Size())

			for {
				kv, err := resMsg.MessageBody.ReadNextKeyValue()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}

					asserter.NoError(err)
				}

				asserter.Contains(messageKVs[i], kv.MappedKey)
				asserter.Equal(messageKVs[i][kv.MappedKey].SerialType, kv.SerialType)
				asserter.Equal(messageKVs[i][kv.MappedKey].Value, kv.Value)
			}
		}
	})
}

func TestReal(t *testing.T) {
	asserter := require.New(t)

	messageFile, err := os.Open("./curqueue.bin")
	asserter.NoError(err)
	defer messageFile.Close()

	messageReader := NewMessageReader(messageFile, make([]byte, 0, 1024))

	for {
		msg, err := messageReader.ReadNextMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			asserter.NoError(err)
		}

		asserter.NotNil(msg.MessageBody)

		for {
			kv, err := msg.MessageBody.ReadNextKeyValue()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				asserter.NoError(err)
			}

			asserter.NotEmpty(kv.MappedKey)

		}
	}
}

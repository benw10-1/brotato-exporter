package messagesubhandler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"log"

	"github.com/benw10-1/brotato-exporter/brotatomod/brotatomodtypes"
	"github.com/benw10-1/brotato-exporter/brotatomod/brotatoserial"
	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/benw10-1/brotato-exporter/exporterserver/ctrlauth"
	"github.com/google/uuid"
)

// AllKeyKey map key which if present will include all keys in the result.
const AllKeyKey = "*"

// MessageSub
type MessageSub struct {
	// subbedKeyMap map for checking if this sub should include a key in the result.
	subbedKeyMap map[string]bool
	messageChan  chan []byte
}

// MessageSubHandler
type MessageSubHandler struct {
	lastMessageReceived map[uuid.UUID]time.Time
	userSubsMap         map[uuid.UUID][]MessageSub
	sessionInfoMap      *ctrlauth.SessionInfoMap // temp hack for resetting state after "disconnect". To avoid having to do a rework already :/
	maxIdleDuration     time.Duration
	// rwmu control reads and writes to userSubsMap
	rwmu sync.RWMutex
}

// NewMessageSubHandler
func NewMessageSubHandler(ctx context.Context, sessionInfoMap *ctrlauth.SessionInfoMap, maxIdleDuration time.Duration) *MessageSubHandler {
	msh := &MessageSubHandler{
		lastMessageReceived: make(map[uuid.UUID]time.Time),
		userSubsMap:         make(map[uuid.UUID][]MessageSub),
		sessionInfoMap:      sessionInfoMap,
		maxIdleDuration:     maxIdleDuration,
	}
	go func() {
		err := msh.sweepIdle(ctx)
		if err != nil {
			log.Printf("messagesubhandler.sweepIdle: returned (%v)", err)
		}
	}()

	return msh
}

// sweepIdle
func (msh *MessageSubHandler) sweepIdle(ctx context.Context) error {
	ticker := time.NewTicker(msh.maxIdleDuration)
	for {
		select {
		case <-ticker.C:
			func() {
				msh.rwmu.Lock()
				defer msh.rwmu.Unlock()

				deleteKeys := make([]uuid.UUID, 0, len(msh.lastMessageReceived))

				for userID, lastKeepAliveTime := range msh.lastMessageReceived {
					if time.Since(lastKeepAliveTime) <= msh.maxIdleDuration {
						continue
					}

					userSubs := msh.userSubsMap[userID]
					for i, sub := range userSubs {
						select {
						case sub.messageChan <- []byte("{}"):
						default:
							log.Printf("messagesubhandler.MessageSubHandler.StreamMessage: messageChan (%d) full for (%s), dropping message", i, userID)
						}
					}

					// reset session state on disconnect as well
					sessInfo, ok := msh.sessionInfoMap.Load(userID)
					if !ok {
						log.Printf("messagesubhandler.MessageSubHandler.StreamMessage: unexpected missing session for (%s)", userID)
						continue
					}

					sessInfo.Lock()

					sessInfo.MessageReader = brotatoserial.NewMessageReader(nil, make([]byte, 1024))
					sessInfo.CurrentSessionState = make(map[string]json.RawMessage)

					sessInfo.Unlock()

					deleteKeys = append(deleteKeys, userID)
				}

				for _, deleteKey := range deleteKeys {
					delete(msh.lastMessageReceived, deleteKey)
				}
			}()
		case <-ctx.Done():
			return errutil.NewStackError(ctx.Err())
		}
	}
}

// StreamMessage
// updateMap will be written to with any key values read - quick hack for now
func (msh *MessageSubHandler) StreamMessage(userID uuid.UUID, updateMap map[string]json.RawMessage, message brotatomodtypes.ExporterMessage) {
	msh.rwmu.Lock()
	defer func() {
		msh.lastMessageReceived[userID] = time.Now()

		msh.rwmu.Unlock()
	}()
	if message.MessageBody == nil || message.MessageBody.Size() == 0 {
		return
	}

	userSubs := msh.userSubsMap[userID]

	subMsgs := make([][]byte, len(userSubs))
	for i := range subMsgs {
		subMsgs[i] = make([]byte, 0, 1024)
		subMsgs[i] = append(subMsgs[i], '{')
	}

	for {
		kv, err := message.MessageBody.ReadNextKeyValue()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			log.Printf("messagesubhandler.MessageSubHandler.StreamMessage: ReadNextKeyValue error: %v", err)
			return
		}

		// reusable JSON representation for both setting the updateMap and building each message
		var jsonRepresentation []byte
		// our own little JSON parser so we don't have to build a sep. map for each sub.
		for i, sub := range userSubs {
			if !sub.subbedKeyMap[AllKeyKey] && !sub.subbedKeyMap[kv.MappedKey] {
				continue
			}

			subMsgs[i] = append(subMsgs[i], '"')
			subMsgs[i] = append(subMsgs[i], kv.MappedKey...)
			subMsgs[i] = append(subMsgs[i], '"', ':')
			if jsonRepresentation == nil {
				startIdx := len(subMsgs[i])
				subMsgs[i] = kv.AppendJSON(subMsgs[i])
				jsonRepresentation = subMsgs[i][startIdx:len(subMsgs[i])]
			} else {
				subMsgs[i] = append(subMsgs[i], jsonRepresentation...)
			}

			subMsgs[i] = append(subMsgs[i], ',')
		}

		if jsonRepresentation == nil {
			jsonRepresentation = kv.AppendJSON(nil)
		} else {
			cpy := make([]byte, len(jsonRepresentation))
			copy(cpy, jsonRepresentation)
			jsonRepresentation = cpy
		}

		updateMap[kv.MappedKey] = jsonRepresentation
	}

	for i, sub := range userSubs {
		if len(subMsgs[i]) < 3 { // needs to be more than "{}"
			continue
		}
		// remove trailing comma
		subMsgs[i][len(subMsgs[i])-1] = '}'
		select {
		case sub.messageChan <- subMsgs[i]:
		default:
			log.Printf("messagesubhandler.MessageSubHandler.StreamMessage: messageChan (%d) full for (%s), dropping message", i, userID)
		}
	}
}

// SubscribeToUser
func (msh *MessageSubHandler) SubscribeToUser(userID uuid.UUID, subbedKeyMap map[string]bool) chan []byte {
	msh.rwmu.Lock()
	defer msh.rwmu.Unlock()

	userSub, ok := msh.userSubsMap[userID]
	if !ok {
		userSub = make([]MessageSub, 0)
	}

	messageChan := make(chan []byte, 1)
	userSub = append(userSub, MessageSub{
		subbedKeyMap: subbedKeyMap,
		messageChan:  messageChan,
	})

	msh.userSubsMap[userID] = userSub

	return messageChan
}

// UnsubscribeFromUser
func (msh *MessageSubHandler) UnsubscribeFromUser(userID uuid.UUID, messageChan chan []byte) {
	msh.rwmu.Lock()
	defer msh.rwmu.Unlock()

	userSubs, ok := msh.userSubsMap[userID]
	if !ok {
		return
	}

	for i, sub := range userSubs {
		if sub.messageChan == messageChan {
			close(sub.messageChan)
			userSubs = append(userSubs[:i], userSubs[i+1:]...)
			break
		}
	}

	if len(userSubs) == 0 {
		delete(msh.userSubsMap, userID)
		return
	}

	msh.userSubsMap[userID] = userSubs
}

// SubscriberCountForUser
func (msh *MessageSubHandler) SubscriberCountForUser(userID uuid.UUID) int {
	msh.rwmu.RLock()
	defer msh.rwmu.RUnlock()

	userSubs, ok := msh.userSubsMap[userID]
	if !ok {
		return 0
	}

	return len(userSubs)
}

// SubscribeToUserIfHasSlots
func (msh *MessageSubHandler) SubscribeToUserIfHasSlots(userID uuid.UUID, subbedKeyMap map[string]bool, maxCount int) (chan []byte, bool) {
	msh.rwmu.Lock()
	defer msh.rwmu.Unlock()

	userSubs, ok := msh.userSubsMap[userID]
	if !ok {
		userSubs = make([]MessageSub, 0)
	}

	if len(userSubs) >= maxCount {
		return nil, false
	}

	// store up to 10 messages before throwing away
	messageChan := make(chan []byte, 10)
	userSubs = append(userSubs, MessageSub{
		subbedKeyMap: subbedKeyMap,
		messageChan:  messageChan,
	})

	msh.userSubsMap[userID] = userSubs

	return messageChan, true
}

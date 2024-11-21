package messagesubhandler

import (
	"encoding/json"
	"errors"
	"io"
	"sync"

	"log"

	"github.com/benw10-1/brotato-exporter/brotatomod/brotatomodtypes"
	"github.com/google/uuid"
)

// MessageSub
type MessageSub struct {
	// subbedKeyMap map for checking if this sub should include a key in the result.
	subbedKeyMap map[string]bool
	messageChan  chan []byte
}

// MessageSubHandler
type MessageSubHandler struct {
	userSubsMap map[uuid.UUID][]MessageSub
	// rwmu control reads and writes to userSubsMap
	rwmu sync.RWMutex
}

// NewMessageSubHandler
func NewMessageSubHandler() *MessageSubHandler {
	return &MessageSubHandler{
		userSubsMap: make(map[uuid.UUID][]MessageSub),
	}
}

// StreamMessage
// updateMap will be written to with any key values read - quick hack for now
func (msh *MessageSubHandler) StreamMessage(userID uuid.UUID, updateMap map[string]json.RawMessage, message brotatomodtypes.ExporterMessage) {
	if message.MessageBody == nil || message.MessageBody.Size() == 0 {
		return
	}

	msh.rwmu.RLock()
	defer msh.rwmu.RUnlock()

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
			if !sub.subbedKeyMap[kv.MappedKey] {
				continue
			}

			subMsgs[i] = append(subMsgs[i], '"')
			subMsgs[i] = append(subMsgs[i], kv.MappedKey...)
			subMsgs[i] = append(subMsgs[i], '"', ':')
			startIdx := len(subMsgs[i])
			subMsgs[i] = kv.AppendJSON(subMsgs[i])
			if jsonRepresentation == nil {
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

	messageChan := make(chan []byte, 1)
	userSubs = append(userSubs, MessageSub{
		subbedKeyMap: subbedKeyMap,
		messageChan:  messageChan,
	})

	msh.userSubsMap[userID] = userSubs

	return messageChan, true
}

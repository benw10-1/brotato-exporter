package ctrlmessage

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/benw10-1/brotato-exporter/exporterserver/ctrlauth"
	"github.com/benw10-1/brotato-exporter/exporterserver/exporterserverutil"
	"github.com/benw10-1/brotato-exporter/exporterserver/messagesubhandler"
	"github.com/benw10-1/brotato-exporter/exporterstore"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

// MessageAPI
type MessageAPI struct {
	sessionInfoMap *ctrlauth.SessionInfoMap

	exporterStore *exporterstore.ExporterStore

	subHandler *messagesubhandler.MessageSubHandler

	router *httprouter.Router
}

// NewMessageAPI
func NewMessageAPI(sessionInfoMap *ctrlauth.SessionInfoMap, exporterStore *exporterstore.ExporterStore, messageSubHandler *messagesubhandler.MessageSubHandler) *MessageAPI {
	router := httprouter.New()
	api := &MessageAPI{
		sessionInfoMap: sessionInfoMap,
		exporterStore:  exporterStore,
		router:         router,
		subHandler:     messageSubHandler,
	}

	router.GET("/api/message/current-state", api.currentState)

	router.POST("/api/message/post", api.receiveMessage)

	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	return api
}

// ServeHTTP
func (api *MessageAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/message/subscribe" {
		api.subscribe(w, r, nil)
		return
	}

	api.router.ServeHTTP(w, r)
}

var byteBufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

// receiveMessage
func (api *MessageAPI) receiveMessage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	exporterserverutil.WriteError(w, func() error {
		sess, ok := ctrlauth.GetSessionFromCtx(r.Context())
		if !ok {
			return exporterserverutil.NewResponseError(nil, http.StatusUnauthorized, "Unauthorized")
		}

		sessInfo, ok := api.sessionInfoMap.Load(sess.UserID)
		if !ok {
			return exporterserverutil.NewResponseError(nil, http.StatusUnauthorized, "Unauthorized")
		}

		if r.Header.Get("Content-Type") != "application/octet-stream" {
			return exporterserverutil.NewResponseError(nil, http.StatusBadRequest, "Invalid content type")
		}

		if r.ContentLength == 0 || r.Body == nil {
			return exporterserverutil.NewResponseError(nil, http.StatusBadRequest, "Invalid content length")
		}

		if sessInfo.MessageReader == nil {
			return exporterserverutil.NewResponseError(nil, http.StatusInternalServerError, "Session message reader not initialized")
		}

		// TODO: write to timeseries file

		bodyReader := byteBufferPool.Get().(*bytes.Buffer)
		defer func() {
			byteBufferPool.Put(bodyReader)
		}()
		bodyReader.Reset()

		// Flush the entire contents of the body to the pooled buffer as the response is likely chunked.
		// Since we are using pool we rarely make any additional allocations
		_, err := io.Copy(bodyReader, r.Body)
		if err != nil {
			return exporterserverutil.NewResponseError(errutil.NewStackError(err), http.StatusInternalServerError, "Failed to read body")
		}

		// make sure we are not setting new reader before old reader has finished reading
		sessInfo.Lock()
		defer sessInfo.Unlock()

		// keep dict encoding as session state, set MessageReader's underlying reader to the incoming body
		sessInfo.MessageReader.SetReader(bodyReader)

		for {
			msg, err := sessInfo.MessageReader.ReadNextMessage()
			if err != nil {
				if errors.Is(err, io.EOF) {
					w.WriteHeader(http.StatusOK)
					return nil
				}

				return errutil.NewStackError(err)
			}
			log.Printf("Received message: %+v", msg)
			api.subHandler.StreamMessage(sess.UserID, sessInfo.CurrentSessionState, msg)
		}
	}())
}

var websocketUpgrader = &websocket.Upgrader{
	// CheckOrigin: func(r *http.Request) bool {
	// 	return true
	// },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// HandshakeTimeout: time.Millisecond * 500,
}

const activityTimeout = time.Minute * 5

// subscribe
func (api *MessageAPI) subscribe(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	userID, ok := ctrlauth.GetUserIDFromCtx(r.Context())
	if !ok {
		exporterserverutil.WriteError(w, exporterserverutil.NewResponseError(nil, http.StatusUnauthorized, "Unauthorized"))
		return
	}

	user, err := api.exporterStore.GetUserByID(userID)
	if err != nil {
		exporterserverutil.WriteError(w, exporterserverutil.NewResponseError(errutil.NewStackError(err), http.StatusInternalServerError, "Failed to get user"))
		return
	}

	queryParams := r.URL.Query()

	subKeyMap := make(map[string]bool, len(queryParams))
	for key, val := range queryParams {
		if len(val) < 1 || (val[len(val)-1] != "1" && val[len(val)-1] != "true") {
			continue
		}

		subKeyMap[key] = true
	}

	if len(subKeyMap) < 1 {
		exporterserverutil.WriteError(w, exporterserverutil.NewResponseError(nil, http.StatusBadRequest, "No valid keys found in query"))
		return
	}

	messageChan, ok := api.subHandler.SubscribeToUserIfHasSlots(user.UserID, subKeyMap, user.MaxSubscribers)
	if !ok {
		exporterserverutil.WriteError(w, exporterserverutil.NewResponseError(nil, http.StatusTooManyRequests, "User has reached max subscribers"))
		return
	}
	defer api.subHandler.UnsubscribeFromUser(user.UserID, messageChan)

	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ctrlmessage.MessageAPI.subscribe: upgrade error: %v", err)
		return
	}
	defer func(conn *websocket.Conn) {
		_ = conn.Close()
	}(conn)

	connErr := func() error {
		// if no activity after 5 minutes, close the connection
		// if the caller is still there they can reconnect
		timeoutTimer := time.NewTimer(activityTimeout)

		for {
			select {
			case msg, ok := <-messageChan:
				if !ok {
					return nil
				}

				err := conn.WriteMessage(websocket.TextMessage, msg)
				if err != nil {
					return errutil.NewStackError(err)
				}

				if !timeoutTimer.Stop() {
					<-timeoutTimer.C
				}
				timeoutTimer.Reset(activityTimeout)
			case <-timeoutTimer.C:
				return errors.New("timer timeout")
			}
		}
	}()
	if connErr != nil {
		log.Printf("ctrlmessage.MessageAPI.subscribe: conn error: %v", connErr)
	}
}

// currentState
func (api *MessageAPI) currentState(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	exporterserverutil.WriteError(w, func() error {
		userID, ok := ctrlauth.GetUserIDFromCtx(r.Context())
		if !ok {
			return exporterserverutil.NewResponseError(nil, http.StatusUnauthorized, "Unauthorized")
		}

		sessInfo, ok := api.sessionInfoMap.Load(userID)
		if !ok {
			return exporterserverutil.NewResponseError(nil, http.StatusNotFound, "No active session found for given auth key")
		}

		sessInfo.Lock()
		defer sessInfo.Unlock()

		w.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(w).Encode(sessInfo.CurrentSessionState)
		if err != nil {
			return exporterserverutil.NewResponseError(errutil.NewStackError(err), http.StatusInternalServerError, "Failed to write JSON")
		}

		w.WriteHeader(http.StatusOK)

		return nil
	}())
}

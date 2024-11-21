package exporterserver

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/benw10-1/brotato-exporter/exporterserver/exporterserverutil"
)

type HandlerNextCtx interface {
	ServeHTTPNextCtx(w http.ResponseWriter, r *http.Request) context.Context
}

// StatusCoder
type StatusCoder interface {
	StatusCode() int
}

// ExporterServer
type ExporterServer struct {
	handlerList []http.Handler

	requestLogEncoder *json.Encoder
}

// NewExporterServer
func NewExporterServer(handlerList []http.Handler, requestLogger *log.Logger) *ExporterServer {
	logger := requestLogger
	if logger == nil {
		logger = log.New(log.Writer(), "http-request", 0)
	}

	server := &ExporterServer{
		handlerList:       handlerList,
		requestLogEncoder: json.NewEncoder(logger.Writer()),
	}

	return server
}

// ServeHTTP
func (es *ExporterServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			trace := debug.Stack()
			log.Printf("exporterserver.ServeHTTP: Recovered from panic: %s -\n%s", r, trace)
		}

		duration := time.Since(startTime)

		statusCoder, ok := w.(StatusCoder)
		if !ok {
			statusCoder = exporterserverutil.NewDummyResponseWriter(w)
		}

		statusCode := statusCoder.StatusCode()
		if statusCode == 0 {
			statusCode = http.StatusNotFound
			if r.Context().Err() != nil {
				statusCode = http.StatusRequestTimeout
			}
			w.WriteHeader(statusCode)
		}

		r.Header.Del("Authorization")

		requestLog := RequestLog{
			Method:      r.Method,
			URL:         r.URL.String(),
			Status:      statusCode,
			Headers:     r.Header,
			Duration:    duration.String(),
			TimeStarted: startTime,
			CtxErr:      r.Context().Err(),
		}

		// log request
		err := es.requestLogEncoder.Encode(requestLog)
		if err != nil {
			log.Printf("exporterserver.ServeHTTP: Error logging request: %s", err)
		}
	}()

	if r.Header.Get("Content-Encoding") == "gzip" {
		w.Header().Add("Content-Encoding", "gzip")
		w = exporterserverutil.NewGzipResponseWriter(w)
	} else {
		w = exporterserverutil.NewDummyResponseWriter(w)
	}

	for _, handler := range es.handlerList {
		nextCtxer, ok := handler.(HandlerNextCtx)
		if ok {
			ctx := nextCtxer.ServeHTTPNextCtx(w, r)
			r = r.WithContext(ctx)
		} else {
			handler.ServeHTTP(w, r)
		}

		if r.Context().Err() != nil {
			return
		}
	}
}

// RequestLog
type RequestLog struct {
	Method      string      `json:"method,omitempty"`
	URL         string      `json:"url,omitempty"`
	Status      int         `json:"status,omitempty"`
	Headers     http.Header `json:"headers,omitempty"`
	Duration    string      `json:"duration,omitempty"`
	TimeStarted time.Time   `json:"time_started,omitempty"`
	CtxErr      error       `json:"ctx_err,omitempty"`
}

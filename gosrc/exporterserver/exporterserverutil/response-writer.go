package exporterserverutil

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"net"
	"net/http"
)

// GzipResponseWriterCloser transparently gzips the response if the client supports it
type GzipResponseWriterCloser struct {
	// underlying writer
	writer http.ResponseWriter

	// gzip writer
	gzipWriter *gzip.Writer
	statusCode int
}

// iface check
var _ http.ResponseWriter = new(GzipResponseWriterCloser)

// NewGzipResponseWriter
func NewGzipResponseWriter(writer http.ResponseWriter) *GzipResponseWriterCloser {
	return &GzipResponseWriterCloser{
		writer:     writer,
		gzipWriter: gzip.NewWriter(writer),
	}
}

// Header
func (grw *GzipResponseWriterCloser) Header() http.Header {
	return grw.writer.Header()
}

// Write
func (grw *GzipResponseWriterCloser) Write(data []byte) (int, error) {
	return grw.gzipWriter.Write(data)
}

// WriteHeader
func (grw *GzipResponseWriterCloser) WriteHeader(statusCode int) {
	grw.writer.WriteHeader(statusCode)
	grw.statusCode = statusCode
}

// StatusCode
func (grw *GzipResponseWriterCloser) StatusCode() int {
	return grw.statusCode
}

// Close
func (grw *GzipResponseWriterCloser) Close() error {
	err := grw.gzipWriter.Close()
	if err != nil {
		return err
	}

	return nil
}

func (grw *GzipResponseWriterCloser) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	err := grw.Close()
	if err != nil {
		return nil, nil, err
	}

	grw.gzipWriter = nil

	if hj, ok := grw.writer.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("ResponseWriter does not implement http.Hijacker")
}

// DummyResponseWriterCloser
type DummyResponseWriterCloser struct {
	writer     http.ResponseWriter
	statusCode int
}

// NewDummyResponseWriter
func NewDummyResponseWriter(writer http.ResponseWriter) *DummyResponseWriterCloser {
	return &DummyResponseWriterCloser{
		writer: writer,
	}
}

// Header
func (drw *DummyResponseWriterCloser) Header() http.Header {
	return drw.writer.Header()
}

// WriteHeader
func (drw *DummyResponseWriterCloser) WriteHeader(statusCode int) {
	drw.statusCode = statusCode
	drw.writer.WriteHeader(statusCode)
}

// Write
func (drw *DummyResponseWriterCloser) Write(data []byte) (int, error) {
	return drw.writer.Write(data)
}

// StatusCode
func (drw *DummyResponseWriterCloser) StatusCode() int {
	return drw.statusCode
}

// Close
func (drw *DummyResponseWriterCloser) Close() error {
	return nil
}

// Hijack
func (drw *DummyResponseWriterCloser) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	err := drw.Close()
	if err != nil {
		return nil, nil, err
	}

	if hj, ok := drw.writer.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("ResponseWriter does not implement http.Hijacker")
}

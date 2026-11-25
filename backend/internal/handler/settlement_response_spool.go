package handler

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const settlementResponseSpoolLimit int64 = 64 << 20

// A settlement response is held in a private temporary file until billing completes.
type settlementResponseSpool struct {
	gin.ResponseWriter
	file         *os.File
	header       http.Header
	status, size int
	stream       bool
	bodyStarted  bool
	sse          bool
	failure      error
}

func newSettlementResponseSpool(writer gin.ResponseWriter, stream bool) (*settlementResponseSpool, error) {
	file, err := os.CreateTemp("", "sub2api-settlement-*")
	if err != nil {
		return nil, err
	}
	// Unlink on Unix while retaining the open descriptor: a process crash cannot
	// leave the response body behind. Other platforms still remove it on close.
	_ = os.Remove(file.Name())
	return &settlementResponseSpool{ResponseWriter: writer, file: file, header: writer.Header().Clone(), status: 200, size: -1, stream: stream}, nil
}
func (w *settlementResponseSpool) Header() http.Header { return w.header }
func (w *settlementResponseSpool) Status() int         { return w.status }
func (w *settlementResponseSpool) Size() int           { return w.size }
func (w *settlementResponseSpool) Written() bool       { return w.size >= 0 }
func (w *settlementResponseSpool) WriteHeader(code int) {
	if !w.Written() {
		w.status = code
	}
}
func (w *settlementResponseSpool) WriteHeaderNow() {
	if w.size < 0 {
		w.size = 0
	}
}
func (w *settlementResponseSpool) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *settlementResponseSpool) Write(b []byte) (int, error) {
	if w.stream && !w.sse && isSSEComment(b) {
		// Queue/upstream keepalives can arrive before the actual protocol is
		// known, or between lines of a JSON fallback. They are not response data.
		return len(b), nil
	}
	w.WriteHeaderNow()
	if w.stream && !w.bodyStarted {
		if first := bytes.TrimSpace(b); len(first) > 0 {
			w.bodyStarted = true
			// An upstream can send a JSON object with an SSE Content-Type.
			// Do not commit SSE headers or comments before seeing the body.
			w.sse = first[0] != '{' && strings.HasPrefix(w.header.Get("Content-Type"), "text/event-stream")
		}
	}
	if int64(w.size)+int64(len(b)) > settlementResponseSpoolLimit {
		w.failure = errors.New("settlement response exceeds 64 MiB")
		return 0, w.failure
	}
	n, err := w.file.Write(b)
	w.size += n
	if err != nil {
		w.failure = err
	}
	return n, err
}

func isSSEComment(body []byte) bool {
	comment := false
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if line[0] != ':' {
			return false
		}
		comment = true
	}
	return comment
}

func (w *settlementResponseSpool) close() { _ = w.file.Close(); _ = os.Remove(w.file.Name()) }
func (w *settlementResponseSpool) commitHeaders() {
	for k, v := range w.header {
		w.ResponseWriter.Header()[k] = v
	}
	w.ResponseWriter.Header().Del("Content-Length")
	w.ResponseWriter.Header().Del("ETag")
	w.ResponseWriter.WriteHeader(w.status)
}

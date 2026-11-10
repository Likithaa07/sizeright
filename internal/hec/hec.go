// Package hec sends log lines to Splunk's HTTP Event Collector in the background.
package hec

import (
	"bytes"
	"crypto/tls"
	"net/http"
	"sync/atomic"
	"time"
)

type Writer struct {
	url, token string
	lines      chan []byte
	client     *http.Client
	Dropped    atomic.Int64 // lines lost because the buffer was full
}

// New starts a background sender. insecureTLS is ONLY for a local Splunk with a
// self-signed certificate. Never use it in production.
func New(url, token string, insecureTLS bool) *Writer {
	w := &Writer{
		url:   url,
		token: token,
		lines: make(chan []byte, 1000), // buffer so logging never blocks requests
		client: &http.Client{
			Timeout:   2 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureTLS}},
		},
	}
	go w.loop()
	return w
}

// Write is called by the logger for every line. It never blocks: if the buffer
// is full (Splunk down or slow), the line is dropped and counted.
func (w *Writer) Write(p []byte) (int, error) {
	line := bytes.TrimSpace(append([]byte(nil), p...))
	select {
	case w.lines <- line:
	default:
		w.Dropped.Add(1)
	}
	return len(p), nil
}

func (w *Writer) loop() {
	for line := range w.lines {
		body := append([]byte(`{"sourcetype":"sizeright","event":`), line...)
		body = append(body, '}')
		w.send(body)
	}
}

// send retries with exponential backoff: wait 100ms, 200ms, 400ms, then give up.
func (w *Writer) send(body []byte) {
	wait := 100 * time.Millisecond
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(http.MethodPost, w.url, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Authorization", "Splunk "+w.token)
		resp, err := w.client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return // success, or a client error that retrying won't fix
			}
		}
		time.Sleep(wait)
		wait *= 2
	}
}

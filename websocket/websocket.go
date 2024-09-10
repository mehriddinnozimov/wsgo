package websocket

import (
	"crypto/sha1"
	"encoding/base64"
	"net/http"

	"github.com/mehriddinnozimov/wsgo/eventemitter"
)

type OnConnection func(socket *Socket)

type WebSocket struct {
	ee *eventemitter.EventEmitter
}

func NewWebSocket() *WebSocket {
	ee := eventemitter.New()
	ws := WebSocket{
		&ee,
	}
	return &ws
}

func (ws *WebSocket) OnConnection(onConnection OnConnection) {
	ws.ee.On("connection", func(sockets ...interface{}) {
		if len(sockets) == 0 {
			return
		}
		socket := sockets[0].(*Socket)
		onConnection(socket)
	})
}

func (ws *WebSocket) Upgrade(w http.ResponseWriter, r *http.Request) {
	upgradeHeader := r.Header.Get("Upgrade")
	if upgradeHeader == "websocket" {
		clientKey := r.Header.Get("Sec-WebSocket-Key")
		if clientKey == "" {
			w.WriteHeader(400)
			return
		}

		socketKey := generateWebSocketKey(clientKey)
		header := w.Header()
		header.Set("Upgrade", "WebSocket")
		header.Set("Connection", "Upgrade")
		header.Set("Sec-WebSocket-Accept", socketKey)
		w.WriteHeader(101)
	} else {
		w.WriteHeader(400)
		return
	}

	wC := http.NewResponseController(w)

	conn, bwr, err := wC.Hijack()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	socket := NewSocket(conn, bwr)

	ws.ee.Emit("connection", socket)
}

func generateWebSocketKey(clientKey string) string {
	const magicString = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(clientKey + magicString))
	hash := h.Sum(nil)
	return base64.StdEncoding.EncodeToString(hash)
}

func setBit(b byte, index int, value bool) byte {
	bitmask := byte(1 << index)

	if value {
		return b | bitmask
	} else {
		return b & ^bitmask
	}
}

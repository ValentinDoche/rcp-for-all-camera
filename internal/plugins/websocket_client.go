package plugins

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// A minimal RFC 6455 client, enough for the camera's event socket: text
// messages, ping/pong and close. It exists to avoid a runtime dependency.

const (
	wsGUID       = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	wsMaxMessage = 1 << 20

	opContinuation = 0x0
	opText         = 0x1
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xA
)

var errWSClosed = errors.New("websocket closed by peer")

// wsConn is one WebSocket connection. client is true when this end masks the
// frames it sends (every real connection); the tests use the server role.
type wsConn struct {
	c      net.Conn
	br     *bufio.Reader
	client bool
	// idle, when set, is how long a read may wait for any frame (pongs
	// included) before it fails.
	idle time.Duration
	wmu  sync.Mutex
}

type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// dialWS opens the socket at ws://hostport+path.
func dialWS(ctx context.Context, dial dialFunc, hostport, path string) (*wsConn, error) {
	c, err := dial(ctx, "tcp", hostport)
	if err != nil {
		return nil, err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = c.SetDeadline(dl)
	}
	w, err := handshake(c, hostport, path)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	_ = c.SetDeadline(time.Time{})
	return w, nil
}

func handshake(c net.Conn, hostport, path string) (*wsConn, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(raw[:])
	req, err := http.NewRequest(http.MethodGet, "http://"+hostport+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", key)
	req.Header.Set("Sec-WebSocket-Version", "13")
	if err := req.Write(c); err != nil {
		return nil, err
	}
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("websocket upgrade refused: HTTP %d", resp.StatusCode)
	}
	if resp.Header.Get("Sec-WebSocket-Accept") != wsAccept(key) {
		return nil, errors.New("websocket upgrade: bad accept key")
	}
	return &wsConn{c: c, br: br, client: true}, nil
}

func wsAccept(key string) string {
	h := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h[:])
}

func (w *wsConn) Close() error { return w.c.Close() }

// writeText sends one text message.
func (w *wsConn) writeText(p []byte) error { return w.writeFrame(opText, p) }

// writePing sends a ping; the peer must answer with a pong.
func (w *wsConn) writePing() error { return w.writeFrame(opPing, nil) }

func (w *wsConn) writeFrame(op byte, p []byte) error {
	hdr := []byte{0x80 | op, 0}
	switch n := len(p); {
	case n < 126:
		hdr[1] = byte(n)
	case n <= 0xFFFF:
		hdr[1] = 126
		hdr = binary.BigEndian.AppendUint16(hdr, uint16(n))
	default:
		hdr[1] = 127
		hdr = binary.BigEndian.AppendUint64(hdr, uint64(n))
	}
	body := p
	if w.client {
		var mask [4]byte
		if _, err := rand.Read(mask[:]); err != nil {
			return err
		}
		hdr[1] |= 0x80
		hdr = append(hdr, mask[:]...)
		body = make([]byte, len(p))
		for i, b := range p {
			body[i] = b ^ mask[i%4]
		}
	}
	w.wmu.Lock()
	defer w.wmu.Unlock()
	if _, err := w.c.Write(append(hdr, body...)); err != nil {
		return err
	}
	return nil
}

// readText returns the next text message, answering pings on the way. It
// returns errWSClosed when the peer closes the socket.
func (w *wsConn) readText() ([]byte, error) {
	var msg []byte
	for {
		fin, op, payload, err := w.readFrame()
		if err != nil {
			return nil, err
		}
		switch op {
		case opPing:
			if err := w.writeFrame(opPong, payload); err != nil {
				return nil, err
			}
		case opPong:
		case opClose:
			_ = w.writeFrame(opClose, nil)
			return nil, errWSClosed
		case opText, opContinuation:
			if len(msg)+len(payload) > wsMaxMessage {
				return nil, errors.New("websocket message too large")
			}
			msg = append(msg, payload...)
			if fin {
				return msg, nil
			}
		}
	}
}

func (w *wsConn) readFrame() (fin bool, op byte, payload []byte, err error) {
	if w.idle > 0 {
		_ = w.c.SetReadDeadline(time.Now().Add(w.idle))
	}
	var h [2]byte
	if _, err = io.ReadFull(w.br, h[:]); err != nil {
		return
	}
	fin, op = h[0]&0x80 != 0, h[0]&0x0F
	masked := h[1]&0x80 != 0
	n := uint64(h[1] & 0x7F)
	switch n {
	case 126:
		var b [2]byte
		if _, err = io.ReadFull(w.br, b[:]); err != nil {
			return
		}
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err = io.ReadFull(w.br, b[:]); err != nil {
			return
		}
		n = binary.BigEndian.Uint64(b[:])
	}
	if n > wsMaxMessage {
		err = errors.New("websocket frame too large")
		return
	}
	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(w.br, mask[:]); err != nil {
			return
		}
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(w.br, payload); err != nil {
		return
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return
}

package api

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// upgradeWS performs a minimal WebSocket upgrade and returns the raw TCP connection.
func upgradeWS(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "not a websocket request", http.StatusBadRequest)
		return nil, fmt.Errorf("not a websocket request")
	}
	key := r.Header.Get("Sec-Websocket-Key")
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return nil, fmt.Errorf("missing key")
	}

	accept := wsAcceptKey(key)

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return nil, fmt.Errorf("hijack not supported")
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		return nil, fmt.Errorf("hijack: %w", err)
	}

	// Send 101 Switching Protocols.
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := buf.WriteString(response); err != nil {
		conn.Close()
		return nil, err
	}
	if err := buf.Flush(); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func wsAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// wsWriteText sends a WebSocket text frame.
func wsWriteText(conn net.Conn, data []byte) error {
	header := make([]byte, 2, 10)
	header[0] = 0x81 // FIN=1, opcode=1 (text)
	length := len(data)
	if length <= 125 {
		header[1] = byte(length)
	} else if length <= 65535 {
		header[1] = 126
		header = append(header, 0, 0)
		binary.BigEndian.PutUint16(header[2:4], uint16(length))
	} else {
		header[1] = 127
		header = append(header, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[2:10], uint64(length))
	}
	frame := append(header, data...)
	_, err := conn.Write(frame)
	return err
}


package modbus

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
)

// WordReader is implemented by the register engine.
type WordReader interface {
	// WordAt returns the uint16 word at the given 0-based address, and whether it exists.
	WordAt(addr uint16) (uint16, bool)
}

// Server is a Modbus TCP server supporting FC01, FC02, FC03, FC04.
type Server struct {
	addr     string
	reader   WordReader
	listener net.Listener
	running  atomic.Bool
	done     chan struct{}
	wg       sync.WaitGroup
}

// New creates a new Modbus TCP server.
func New(addr string, reader WordReader) *Server {
	return &Server{
		addr:   addr,
		reader: reader,
		done:   make(chan struct{}),
	}
}

// Start begins listening for Modbus TCP connections.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("modbus listen %s: %w", s.addr, err)
	}
	s.listener = ln
	s.running.Store(true)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-s.done:
					return
				default:
					slog.Warn("modbus: accept error", "err", err)
					return
				}
			}
			go s.handleConn(conn)
		}
	}()

	slog.Info("modbus server started", "addr", s.addr)
	return nil
}

// Stop shuts down the server gracefully.
func (s *Server) Stop() {
	if !s.running.CompareAndSwap(true, false) {
		return
	}
	close(s.done)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 260)
	for {
		// Read 7-byte MBAP header.
		if _, err := io.ReadFull(conn, buf[:7]); err != nil {
			return
		}
		transID := [2]byte{buf[0], buf[1]}
		// protocol ID buf[2:4] — ignored (should be 0)
		length := binary.BigEndian.Uint16(buf[4:6])
		if length < 2 || length > 253 {
			return
		}
		pduLen := int(length) - 1 // length includes unit ID byte
		if pduLen < 1 {
			return
		}
		if _, err := io.ReadFull(conn, buf[7:7+pduLen]); err != nil {
			return
		}

		fc := buf[7]
		pduData := buf[8 : 7+pduLen]
		var resp []byte

		switch fc {
		case 0x01: // Read Coils
			resp = s.fcReadBits(transID, 0x01, pduData)
		case 0x02: // Read Discrete Inputs
			resp = s.fcReadBits(transID, 0x02, pduData)
		case 0x03: // Read Holding Registers
			resp = s.fcReadWords(transID, 0x03, pduData)
		case 0x04: // Read Input Registers
			resp = s.fcReadWords(transID, 0x04, pduData)
		default:
			resp = s.mbapFrame(transID, []byte{fc | 0x80, 0x01}) // illegal function
		}

		if _, err := conn.Write(resp); err != nil {
			return
		}
	}
}

// fcReadWords handles FC03 and FC04 (read word registers).
func (s *Server) fcReadWords(transID [2]byte, fc byte, data []byte) []byte {
	if len(data) < 4 {
		return s.mbapFrame(transID, []byte{fc | 0x80, 0x03})
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	if quantity == 0 || quantity > 125 {
		return s.mbapFrame(transID, []byte{fc | 0x80, 0x03})
	}

	regBytes := make([]byte, quantity*2)
	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		word, _ := s.reader.WordAt(addr)
		binary.BigEndian.PutUint16(regBytes[i*2:], word)
	}

	pdu := append([]byte{fc, byte(quantity * 2)}, regBytes...)
	return s.mbapFrame(transID, pdu)
}

// fcReadBits handles FC01 (coils) and FC02 (discrete inputs).
// Each word address maps to one coil/input: nonzero word = 1, zero = 0.
func (s *Server) fcReadBits(transID [2]byte, fc byte, data []byte) []byte {
	if len(data) < 4 {
		return s.mbapFrame(transID, []byte{fc | 0x80, 0x03})
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	if quantity == 0 || quantity > 2000 {
		return s.mbapFrame(transID, []byte{fc | 0x80, 0x03})
	}

	byteCount := (quantity + 7) / 8
	coilBytes := make([]byte, byteCount)
	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		word, _ := s.reader.WordAt(addr)
		if word != 0 {
			coilBytes[i/8] |= 1 << (i % 8)
		}
	}

	pdu := append([]byte{fc, byte(byteCount)}, coilBytes...)
	return s.mbapFrame(transID, pdu)
}

func (s *Server) mbapFrame(transID [2]byte, pdu []byte) []byte {
	frame := make([]byte, 7+len(pdu))
	frame[0], frame[1] = transID[0], transID[1]
	// protocol ID = 0 (already zero)
	binary.BigEndian.PutUint16(frame[4:6], uint16(1+len(pdu)))
	frame[6] = 0x01 // unit ID
	copy(frame[7:], pdu)
	return frame
}

package modbus

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// ─── Stub WordReader ──────────────────────────────────────────────────────────

type stubReader struct {
	words map[uint16]uint16
}

func (s *stubReader) WordAt(addr uint16) (uint16, bool) {
	v, ok := s.words[addr]
	return v, ok
}

func newStub(words map[uint16]uint16) *stubReader {
	return &stubReader{words: words}
}

// ─── test helpers ─────────────────────────────────────────────────────────────

// buildRequest builds a Modbus TCP request frame.
func buildRequest(transID uint16, fc byte, startAddr, quantity uint16) []byte {
	pdu := make([]byte, 5)
	pdu[0] = fc
	binary.BigEndian.PutUint16(pdu[1:3], startAddr)
	binary.BigEndian.PutUint16(pdu[3:5], quantity)

	frame := make([]byte, 7+len(pdu))
	binary.BigEndian.PutUint16(frame[0:2], transID)
	// protocol ID = 0
	binary.BigEndian.PutUint16(frame[4:6], uint16(1+len(pdu))) // length = unitID + PDU
	frame[6] = 0x01                                             // unit ID
	copy(frame[7:], pdu)
	return frame
}

// sendRecv sends a Modbus frame and returns the response.
func sendRecv(t *testing.T, conn net.Conn, frame []byte) []byte {
	t.Helper()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 260)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return buf[:n]
}

// startServer starts a Modbus server on a random port and returns its address.
func startServer(t *testing.T, reader WordReader) *Server {
	t.Helper()
	srv := New("127.0.0.1:0", reader)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Patch addr to the actual bound address.
	srv.addr = srv.listener.Addr().String()
	t.Cleanup(srv.Stop)
	return srv
}

// ─── FC03: Read Holding Registers ────────────────────────────────────────────

func TestFC03ReadHoldingRegisters(t *testing.T) {
	reader := newStub(map[uint16]uint16{0: 100, 1: 200, 2: 300})
	srv := startServer(t, reader)

	conn, err := net.Dial("tcp", srv.addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	resp := sendRecv(t, conn, buildRequest(1, 0x03, 0, 3))
	// MBAP(7) + FC(1) + ByteCount(1) + 3 words (6 bytes) = 15
	if len(resp) < 9 {
		t.Fatalf("response too short: %d bytes", len(resp))
	}
	fc := resp[7]
	if fc != 0x03 {
		t.Errorf("FC = 0x%02X, want 0x03", fc)
	}
	byteCount := resp[8]
	if byteCount != 6 {
		t.Errorf("byteCount = %d, want 6", byteCount)
	}
	w0 := binary.BigEndian.Uint16(resp[9:11])
	w1 := binary.BigEndian.Uint16(resp[11:13])
	w2 := binary.BigEndian.Uint16(resp[13:15])
	if w0 != 100 || w1 != 200 || w2 != 300 {
		t.Errorf("words = %d %d %d, want 100 200 300", w0, w1, w2)
	}
}

func TestFC03MissingAddress(t *testing.T) {
	// Addresses not in reader return 0.
	reader := newStub(map[uint16]uint16{})
	srv := startServer(t, reader)

	conn, _ := net.Dial("tcp", srv.addr)
	defer conn.Close()

	resp := sendRecv(t, conn, buildRequest(1, 0x03, 5, 1))
	if resp[7] != 0x03 {
		t.Errorf("expected FC 0x03, got 0x%02X", resp[7])
	}
	word := binary.BigEndian.Uint16(resp[9:11])
	if word != 0 {
		t.Errorf("missing register word = %d, want 0", word)
	}
}

// ─── FC04: Read Input Registers ──────────────────────────────────────────────

func TestFC04ReadInputRegisters(t *testing.T) {
	reader := newStub(map[uint16]uint16{10: 999})
	srv := startServer(t, reader)

	conn, _ := net.Dial("tcp", srv.addr)
	defer conn.Close()

	resp := sendRecv(t, conn, buildRequest(2, 0x04, 10, 1))
	if resp[7] != 0x04 {
		t.Errorf("FC = 0x%02X, want 0x04", resp[7])
	}
	word := binary.BigEndian.Uint16(resp[9:11])
	if word != 999 {
		t.Errorf("word = %d, want 999", word)
	}
}

// ─── FC01: Read Coils ─────────────────────────────────────────────────────────

func TestFC01ReadCoils(t *testing.T) {
	// addr 0 = nonzero (coil ON), addr 1 = 0 (coil OFF)
	reader := newStub(map[uint16]uint16{0: 1, 1: 0})
	srv := startServer(t, reader)

	conn, _ := net.Dial("tcp", srv.addr)
	defer conn.Close()

	resp := sendRecv(t, conn, buildRequest(3, 0x01, 0, 2))
	if resp[7] != 0x01 {
		t.Errorf("FC = 0x%02X, want 0x01", resp[7])
	}
	byteCount := resp[8]
	if byteCount != 1 {
		t.Errorf("byteCount = %d, want 1", byteCount)
	}
	coilByte := resp[9]
	// Coil 0 is bit 0 (1), coil 1 is bit 1 (0) => 0b00000001 = 0x01
	if coilByte != 0x01 {
		t.Errorf("coilByte = 0x%02X, want 0x01", coilByte)
	}
}

// ─── FC02: Read Discrete Inputs ──────────────────────────────────────────────

func TestFC02ReadDiscreteInputs(t *testing.T) {
	reader := newStub(map[uint16]uint16{0: 0, 1: 5, 2: 0})
	srv := startServer(t, reader)

	conn, _ := net.Dial("tcp", srv.addr)
	defer conn.Close()

	resp := sendRecv(t, conn, buildRequest(4, 0x02, 0, 3))
	if resp[7] != 0x02 {
		t.Errorf("FC = 0x%02X, want 0x02", resp[7])
	}
	// bits: addr0=0, addr1=1, addr2=0 => 0b00000010 = 0x02
	coilByte := resp[9]
	if coilByte != 0x02 {
		t.Errorf("coilByte = 0x%02X, want 0x02", coilByte)
	}
}

// ─── Illegal function code ────────────────────────────────────────────────────

func TestIllegalFunctionCode(t *testing.T) {
	reader := newStub(map[uint16]uint16{})
	srv := startServer(t, reader)

	conn, _ := net.Dial("tcp", srv.addr)
	defer conn.Close()

	resp := sendRecv(t, conn, buildRequest(5, 0x10, 0, 1))
	// Response FC should be 0x10 | 0x80 = 0x90
	if resp[7] != 0x90 {
		t.Errorf("error FC = 0x%02X, want 0x90", resp[7])
	}
	// Exception code = 0x01 (illegal function)
	if resp[8] != 0x01 {
		t.Errorf("exception code = 0x%02X, want 0x01", resp[8])
	}
}

// ─── Quantity validation ──────────────────────────────────────────────────────

func TestFC03QuantityZero(t *testing.T) {
	reader := newStub(map[uint16]uint16{0: 1})
	srv := startServer(t, reader)

	conn, _ := net.Dial("tcp", srv.addr)
	defer conn.Close()

	resp := sendRecv(t, conn, buildRequest(6, 0x03, 0, 0))
	// Exception response: FC | 0x80 = 0x83
	if resp[7] != 0x83 {
		t.Errorf("FC = 0x%02X, want 0x83 for zero quantity", resp[7])
	}
}

func TestFC03QuantityTooLarge(t *testing.T) {
	reader := newStub(map[uint16]uint16{})
	srv := startServer(t, reader)

	conn, _ := net.Dial("tcp", srv.addr)
	defer conn.Close()

	resp := sendRecv(t, conn, buildRequest(7, 0x03, 0, 126))
	if resp[7] != 0x83 {
		t.Errorf("FC = 0x%02X, want 0x83 for quantity > 125", resp[7])
	}
}

// ─── Stop ─────────────────────────────────────────────────────────────────────

func TestStopPreventsNewConnections(t *testing.T) {
	reader := newStub(map[uint16]uint16{})
	srv := New("127.0.0.1:0", reader)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	addr := srv.listener.Addr().String()
	srv.addr = addr
	srv.Stop()

	// After stop, new connections should fail.
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("expected connection to fail after Stop()")
	}
}

func TestStopIdempotent(t *testing.T) {
	reader := newStub(map[uint16]uint16{})
	srv := New("127.0.0.1:0", reader)
	srv.Start()
	srv.Stop()
	srv.Stop() // should not panic
}

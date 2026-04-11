package register

import (
	"context"
	"math"
	"testing"
	"time"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func newReg(id string, addr uint16, dt DataType, sig Signal) Register {
	return Register{
		ID:       id,
		Name:     id,
		Address:  addr,
		DataType: dt,
		Signal:   sig,
	}
}

// ─── Add ─────────────────────────────────────────────────────────────────────

func TestEngineAddBasic(t *testing.T) {
	e := NewEngine()
	id, err := e.Add(newReg("r1", 0, TypeUint16, Signal{Kind: SignalConstant, Value: 42}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "r1" {
		t.Errorf("id = %q, want %q", id, "r1")
	}
}

func TestEngineAddMissingName(t *testing.T) {
	e := NewEngine()
	r := Register{ID: "r1", Address: 0, DataType: TypeUint16}
	_, err := e.Add(r)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestEngineAddDuplicateID(t *testing.T) {
	e := NewEngine()
	r := newReg("r1", 0, TypeUint16, Signal{Kind: SignalConstant})
	if _, err := e.Add(r); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	r2 := newReg("r1", 5, TypeUint16, Signal{Kind: SignalConstant})
	if _, err := e.Add(r2); err == nil {
		t.Fatal("expected error for duplicate ID")
	}
}

func TestEngineAddAddressConflict(t *testing.T) {
	e := NewEngine()
	// float32 occupies addresses 0 and 1
	if _, err := e.Add(newReg("r1", 0, TypeFloat32, Signal{Kind: SignalConstant})); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	// uint16 at address 1 should conflict
	if _, err := e.Add(newReg("r2", 1, TypeUint16, Signal{Kind: SignalConstant})); err == nil {
		t.Fatal("expected address conflict error")
	}
}

func TestEngineAddAutoID(t *testing.T) {
	e := NewEngine()
	r := Register{Name: "NoID", Address: 3, DataType: TypeUint16, Signal: Signal{Kind: SignalConstant}}
	id, err := e.Add(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty auto-generated ID")
	}
}

func TestEngineAddDefaultDataType(t *testing.T) {
	e := NewEngine()
	r := Register{ID: "r1", Name: "r1", Address: 0, Signal: Signal{Kind: SignalConstant}}
	if _, err := e.Add(r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := e.Get("r1")
	if got.DataType != TypeUint16 {
		t.Errorf("DataType = %q, want %q", got.DataType, TypeUint16)
	}
}

// ─── Get / List ───────────────────────────────────────────────────────────────

func TestEngineGetExisting(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeUint16, Signal{Kind: SignalConstant, Value: 7}))
	r, ok := e.Get("r1")
	if !ok {
		t.Fatal("Get returned false for existing register")
	}
	if r.ID != "r1" {
		t.Errorf("ID = %q, want %q", r.ID, "r1")
	}
}

func TestEngineGetMissing(t *testing.T) {
	e := NewEngine()
	_, ok := e.Get("missing")
	if ok {
		t.Fatal("Get returned true for missing register")
	}
}

func TestEngineList(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeUint16, Signal{Kind: SignalConstant}))
	e.Add(newReg("r2", 1, TypeUint16, Signal{Kind: SignalConstant}))
	list := e.List()
	if len(list) != 2 {
		t.Errorf("List len = %d, want 2", len(list))
	}
}

// ─── Remove ───────────────────────────────────────────────────────────────────

func TestEngineRemoveExisting(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeUint16, Signal{Kind: SignalConstant}))
	if err := e.Remove("r1"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if len(e.List()) != 0 {
		t.Error("expected empty list after remove")
	}
}

func TestEngineRemoveMissing(t *testing.T) {
	e := NewEngine()
	if err := e.Remove("ghost"); err == nil {
		t.Fatal("expected error removing missing register")
	}
}

// ─── Update ───────────────────────────────────────────────────────────────────

func TestEngineUpdate(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeUint16, Signal{Kind: SignalConstant, Value: 1}))
	updated := newReg("r1", 0, TypeUint16, Signal{Kind: SignalConstant, Value: 99})
	if err := e.Update("r1", updated); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
}

func TestEngineUpdateMissing(t *testing.T) {
	e := NewEngine()
	if err := e.Update("ghost", newReg("ghost", 0, TypeUint16, Signal{})); err == nil {
		t.Fatal("expected error updating missing register")
	}
}

func TestEngineUpdateAddressConflict(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeFloat32, Signal{Kind: SignalConstant})) // occupies 0,1
	e.Add(newReg("r2", 2, TypeUint16, Signal{Kind: SignalConstant}))
	// Try to move r2 to address 1 (conflicts with r1)
	if err := e.Update("r2", newReg("r2", 1, TypeUint16, Signal{})); err == nil {
		t.Fatal("expected address conflict error on update")
	}
}

// ─── WordAt ───────────────────────────────────────────────────────────────────

func TestEngineWordAtUint16(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 5, TypeUint16, Signal{Kind: SignalConstant, Value: 1234}))
	// Trigger a tick so value gets written.
	e.tick()
	word, ok := e.WordAt(5)
	if !ok {
		t.Fatal("WordAt returned false for known address")
	}
	if word != 1234 {
		t.Errorf("WordAt(5) = %d, want 1234", word)
	}
}

func TestEngineWordAtMissing(t *testing.T) {
	e := NewEngine()
	_, ok := e.WordAt(99)
	if ok {
		t.Fatal("WordAt returned true for unknown address")
	}
}

func TestEngineWordAtFloat32(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeFloat32, Signal{Kind: SignalConstant, Value: 1.5}))
	e.tick()
	w0, ok0 := e.WordAt(0)
	w1, ok1 := e.WordAt(1)
	if !ok0 || !ok1 {
		t.Fatal("expected both words to exist for float32")
	}
	bits := uint32(w0)<<16 | uint32(w1)
	got := math.Float32frombits(bits)
	if math.Abs(float64(got)-1.5) > 0.001 {
		t.Errorf("decoded float32 = %v, want 1.5", got)
	}
}

// ─── encodeValue ─────────────────────────────────────────────────────────────

func TestEncodeValue(t *testing.T) {
	tests := []struct {
		dt   DataType
		v    float64
		want []uint16
	}{
		{TypeUint16, 42, []uint16{42}},
		{TypeInt16, -1, []uint16{0xFFFF}},
		{TypeBool, 1, []uint16{1}},
		{TypeBool, 0, []uint16{0}},
		{TypeUint32, 65536, []uint16{0, 1}},
		{TypeInt32, -1, []uint16{0xFFFF, 0xFFFF}},
	}
	for _, tc := range tests {
		got := encodeValue(tc.v, tc.dt)
		if len(got) != len(tc.want) {
			t.Errorf("encodeValue(%v, %v) len=%d want %d", tc.v, tc.dt, len(got), len(tc.want))
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("encodeValue(%v, %v)[%d] = %d, want %d", tc.v, tc.dt, i, got[i], tc.want[i])
			}
		}
	}
}

// ─── Signal simulation via tick ───────────────────────────────────────────────

func TestTickConstant(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeUint16, Signal{Kind: SignalConstant, Value: 55}))
	e.tick()
	r, _ := e.Get("r1")
	if r.Value != 55 {
		t.Errorf("constant value = %v, want 55", r.Value)
	}
}

func TestTickSine(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeFloat32, Signal{
		Kind:      SignalSine,
		Amplitude: 10,
		Period:    10,
		Offset:    20,
		Min:       10,
		Max:       30,
	}))
	e.tick()
	r, _ := e.Get("r1")
	if r.Value < 10 || r.Value > 30 {
		t.Errorf("sine value %v out of [10, 30]", r.Value)
	}
}

func TestTickRamp(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeFloat32, Signal{
		Kind: SignalRamp,
		Rate: 1,
		Min:  0,
		Max:  100,
	}))
	e.tick()
	r, _ := e.Get("r1")
	if r.Value < 0 || r.Value > 100 {
		t.Errorf("ramp value %v out of [0, 100]", r.Value)
	}
}

func TestTickStep(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeFloat32, Signal{
		Kind:   SignalStep,
		Low:    0,
		High:   1,
		Period: 5,
	}))
	e.tick()
	r, _ := e.Get("r1")
	if r.Value != 0 && r.Value != 1 {
		t.Errorf("step value %v not in {0, 1}", r.Value)
	}
}

func TestTickRandomWalk(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeFloat32, Signal{
		Kind:        SignalRandomWalk,
		StepMaxWalk: 1,
		Min:         0,
		Max:         100,
	}))
	for i := 0; i < 20; i++ {
		e.tick()
	}
	r, _ := e.Get("r1")
	if r.Value < 0 || r.Value > 100 {
		t.Errorf("random_walk value %v out of [0, 100]", r.Value)
	}
}

// ─── Counter ──────────────────────────────────────────────────────────────────

func TestCounterIncrement(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("c1", 0, TypeUint16, Signal{
		Kind:       SignalCounter,
		StepMin:    1,
		IntervalMs: 50,
		Min:        0,
		Max:        10,
	}))
	time.Sleep(160 * time.Millisecond)
	r, _ := e.Get("c1")
	if r.Value < 1 {
		t.Errorf("counter value = %v, expected >= 1", r.Value)
	}
}

func TestCounterWrap(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("c1", 0, TypeUint16, Signal{
		Kind:       SignalCounter,
		StepMin:    5,
		IntervalMs: 50,
		Min:        0,
		Max:        10,
	}))
	time.Sleep(300 * time.Millisecond)
	r, _ := e.Get("c1")
	if r.Value > 10 {
		t.Errorf("counter value %v exceeded max 10", r.Value)
	}
}

// ─── Subscribe / Unsubscribe ─────────────────────────────────────────────────

func TestSubscribeReceivesSnapshots(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeUint16, Signal{Kind: SignalConstant, Value: 1}))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	e.Start(ctx)

	ch := e.Subscribe()
	defer e.Unsubscribe(ch)

	select {
	case snaps := <-ch:
		if len(snaps) != 1 {
			t.Errorf("expected 1 snapshot, got %d", len(snaps))
		}
	case <-time.After(400 * time.Millisecond):
		t.Fatal("timed out waiting for snapshot")
	}
}

func TestUnsubscribeRemovesChannel(t *testing.T) {
	e := NewEngine()
	ch := e.Subscribe()
	e.Unsubscribe(ch)

	e.subMu.RLock()
	n := len(e.subscribers)
	e.subMu.RUnlock()

	if n != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", n)
	}
}

// ─── Encode ───────────────────────────────────────────────────────────────────

func TestEngineEncode(t *testing.T) {
	e := NewEngine()
	e.Add(newReg("r1", 0, TypeUint16, Signal{Kind: SignalConstant, Value: 7}))
	e.tick()
	words, err := e.Encode("r1")
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if len(words) != 1 || words[0] != 7 {
		t.Errorf("Encode = %v, want [7]", words)
	}
}

func TestEngineEncodeMissing(t *testing.T) {
	e := NewEngine()
	if _, err := e.Encode("ghost"); err == nil {
		t.Fatal("expected error encoding missing register")
	}
}

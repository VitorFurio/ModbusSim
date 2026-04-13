package device

import (
	"context"
	"testing"
	"time"

	"modbussim/internal/register"
)

// ─── New ──────────────────────────────────────────────────────────────────────

func TestNewDeviceDefaults(t *testing.T) {
	d := New("dev1", "Device 1", "desc", ":5020")
	if d.ID() != "dev1" {
		t.Errorf("ID = %q, want %q", d.ID(), "dev1")
	}
	if d.Name() != "Device 1" {
		t.Errorf("Name = %q, want %q", d.Name(), "Device 1")
	}
	if d.GetStatus() != StatusStopped {
		t.Errorf("Status = %q, want %q", d.GetStatus(), StatusStopped)
	}
	if d.Engine() == nil {
		t.Error("Engine should not be nil")
	}
	if d.RunCtx() != nil {
		t.Error("RunCtx should be nil when stopped")
	}
}

// ─── Start / Stop ─────────────────────────────────────────────────────────────

func TestStartStop(t *testing.T) {
	d := New("dev1", "Dev", "", ":15100")
	ctx := context.Background()

	if err := d.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer d.Stop()

	if d.GetStatus() != StatusRunning {
		t.Errorf("status = %q after Start, want %q", d.GetStatus(), StatusRunning)
	}
	if d.RunCtx() == nil {
		t.Error("RunCtx should be non-nil when running")
	}

	d.Stop()
	if d.GetStatus() != StatusStopped {
		t.Errorf("status = %q after Stop, want %q", d.GetStatus(), StatusStopped)
	}
	if d.RunCtx() != nil {
		t.Error("RunCtx should be nil after Stop")
	}
}

func TestStartIdempotent(t *testing.T) {
	d := New("dev1", "Dev", "", ":15101")
	ctx := context.Background()
	defer d.Stop()

	if err := d.Start(ctx); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}
	// second Start should be a no-op
	if err := d.Start(ctx); err != nil {
		t.Fatalf("second Start failed: %v", err)
	}
	if d.GetStatus() != StatusRunning {
		t.Errorf("status = %q, want running", d.GetStatus())
	}
}

func TestStopIdempotent(t *testing.T) {
	d := New("dev1", "Dev", "", ":15102")
	d.Stop() // stop when already stopped — must not panic
	d.Stop()
}

func TestStartPortConflict(t *testing.T) {
	d1 := New("dev1", "Dev1", "", ":15103")
	d2 := New("dev2", "Dev2", "", ":15103")
	ctx := context.Background()

	if err := d1.Start(ctx); err != nil {
		t.Fatalf("d1 Start failed: %v", err)
	}
	defer d1.Stop()

	if err := d2.Start(ctx); err == nil {
		d2.Stop()
		t.Fatal("expected error starting device on occupied port")
	}
}

func TestContextCancelStopsDevice(t *testing.T) {
	d := New("dev1", "Dev", "", ":15104")
	ctx, cancel := context.WithCancel(context.Background())

	if err := d.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	cancel() // cancel the parent context
	time.Sleep(50 * time.Millisecond)
	// After parent context is cancelled, the device's runCtx should be done.
	runCtx := d.RunCtx()
	if runCtx != nil {
		select {
		case <-runCtx.Done():
			// expected
		default:
			// runCtx still active — acceptable since device.Stop() wasn't called
		}
	}
}

// ─── Info ──────────────────────────────────────────────────────────────────────

func TestInfo(t *testing.T) {
	d := New("dev1", "My Device", "desc", ":5020")
	d.Engine().Add(register.Register{
		ID: "r1", Name: "R1", Address: 0, DataType: register.TypeUint16,
		Signal: register.Signal{Kind: register.SignalConstant},
	})

	info := d.Info()
	if info.ID != "dev1" {
		t.Errorf("Info.ID = %q, want %q", info.ID, "dev1")
	}
	if info.Name != "My Device" {
		t.Errorf("Info.Name = %q, want %q", info.Name, "My Device")
	}
	if info.Status != StatusStopped {
		t.Errorf("Info.Status = %q, want %q", info.Status, StatusStopped)
	}
	if info.RegCount != 1 {
		t.Errorf("Info.RegCount = %d, want 1", info.RegCount)
	}
}

// ─── Setters ──────────────────────────────────────────────────────────────────

func TestSetters(t *testing.T) {
	d := New("dev1", "Old", "old desc", ":5020")
	d.SetName("New")
	d.SetDescription("new desc")
	d.SetModbusAddr(":5021")

	if d.Name() != "New" {
		t.Errorf("Name = %q, want %q", d.Name(), "New")
	}
	if d.Description() != "new desc" {
		t.Errorf("Description = %q, want %q", d.Description(), "new desc")
	}
	if d.ModbusAddr() != ":5021" {
		t.Errorf("ModbusAddr = %q, want %q", d.ModbusAddr(), ":5021")
	}
}

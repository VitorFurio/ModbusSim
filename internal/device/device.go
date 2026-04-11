package device

import (
	"context"
	"fmt"
	"sync"

	"modbussim/internal/modbus"
	"modbussim/internal/register"
)

// Status represents whether a device is running or stopped.
type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
)

// Info is the wire shape returned to API clients.
type Info struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ModbusAddr  string `json:"modbus_addr"`
	Status      Status `json:"status"`
	RegCount    int    `json:"reg_count"`
}

// Device is a simulation unit with its own engine and Modbus TCP server.
type Device struct {
	mu        sync.RWMutex
	id        string
	name      string
	desc      string
	addr      string
	engine    *register.Engine
	srv       *modbus.Server
	status    Status
	runCtx    context.Context
	runCancel context.CancelFunc
}

// New creates a Device in stopped state with an empty engine.
func New(id, name, desc, modbusAddr string) *Device {
	return &Device{
		id:     id,
		name:   name,
		desc:   desc,
		addr:   modbusAddr,
		engine: register.NewEngine(),
		status: StatusStopped,
	}
}

// ID returns the device's stable identifier.
func (d *Device) ID() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.id
}

// Name returns the display name.
func (d *Device) Name() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.name
}

// Description returns the optional description.
func (d *Device) Description() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.desc
}

// ModbusAddr returns the configured Modbus TCP address.
func (d *Device) ModbusAddr() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.addr
}

// GetStatus returns running/stopped.
func (d *Device) GetStatus() Status {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.status
}

// Engine returns the underlying register engine.
func (d *Device) Engine() *register.Engine {
	return d.engine
}

// RunCtx returns the context that is active while the device is running.
// Returns nil if the device is stopped.
func (d *Device) RunCtx() context.Context {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.runCtx
}

// SetName updates the display name (does not persist to disk).
func (d *Device) SetName(name string) {
	d.mu.Lock()
	d.name = name
	d.mu.Unlock()
}

// SetDescription updates the description.
func (d *Device) SetDescription(desc string) {
	d.mu.Lock()
	d.desc = desc
	d.mu.Unlock()
}

// SetModbusAddr updates the Modbus TCP address. The device should be stopped first.
func (d *Device) SetModbusAddr(addr string) {
	d.mu.Lock()
	d.addr = addr
	d.mu.Unlock()
}

// Start launches the engine tick loop and the Modbus TCP server.
// If the device is already running, this is a no-op.
func (d *Device) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.status == StatusRunning {
		return nil
	}

	childCtx, cancel := context.WithCancel(ctx)
	d.engine.Start(childCtx)

	srv := modbus.New(d.addr, d.engine)
	if err := srv.Start(); err != nil {
		cancel()
		return fmt.Errorf("modbus listen %s: %w", d.addr, err)
	}

	d.runCtx = childCtx
	d.runCancel = cancel
	d.srv = srv
	d.status = StatusRunning
	return nil
}

// Stop halts the engine and closes the Modbus TCP listener.
// If the device is already stopped, this is a no-op.
func (d *Device) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.status == StatusStopped {
		return
	}

	if d.runCancel != nil {
		d.runCancel()
		d.runCancel = nil
	}
	d.runCtx = nil

	if d.srv != nil {
		d.srv.Stop()
		d.srv = nil
	}
	d.status = StatusStopped
}

// Info returns a snapshot of the device's current state.
func (d *Device) Info() Info {
	d.mu.RLock()
	id := d.id
	name := d.name
	desc := d.desc
	addr := d.addr
	status := d.status
	d.mu.RUnlock()

	return Info{
		ID:          id,
		Name:        name,
		Description: desc,
		ModbusAddr:  addr,
		Status:      status,
		RegCount:    len(d.engine.List()),
	}
}

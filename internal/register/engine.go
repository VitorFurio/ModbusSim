package register

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// RegisterSnapshot is a lightweight value snapshot sent over the broadcast channel.
type RegisterSnapshot struct {
	ID        string      `json:"id"`
	Value     float64     `json:"value"`
	UpdatedAt int64       `json:"updated_at"`
	History   [30]float64 `json:"history"`
}

type runtimeRegister struct {
	reg        *Register
	startTime  time.Time
	cancel     context.CancelFunc // for counter goroutines
	history    [30]float64
	historyIdx int
	mu         sync.Mutex
}

func (rr *runtimeRegister) addHistory(v float64) {
	rr.history[rr.historyIdx%30] = v
	rr.historyIdx++
}

func (rr *runtimeRegister) getHistory() [30]float64 {
	return rr.history
}

// Engine manages all registers and drives signal simulation.
type Engine struct {
	mu          sync.RWMutex
	registers   []*runtimeRegister
	idIndex     map[string]*runtimeRegister
	subscribers []chan []RegisterSnapshot
	subMu       sync.RWMutex
}

// NewEngine creates a new Engine.
func NewEngine() *Engine {
	return &Engine{
		idIndex: make(map[string]*runtimeRegister),
	}
}

// Start launches the main 100ms tick loop. Runs until ctx is cancelled.
func (e *Engine) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.tick()
				e.broadcast()
			}
		}
	}()
}

func (e *Engine) tick() {
	now := time.Now()
	e.mu.RLock()
	regs := make([]*runtimeRegister, len(e.registers))
	copy(regs, e.registers)
	e.mu.RUnlock()

	for _, rr := range regs {
		rr.mu.Lock()
		t := now.Sub(rr.startTime).Seconds()
		sig := rr.reg.Signal
		var v float64

		switch sig.Kind {
		case SignalConstant:
			v = sig.Value
		case SignalSine:
			p := sig.Period
			if p <= 0 {
				p = 10
			}
			v = sig.Amplitude*math.Sin(2*math.Pi*t/p) + sig.Offset
			if sig.Max > sig.Min {
				if v < sig.Min {
					v = sig.Min
				}
				if v > sig.Max {
					v = sig.Max
				}
			}
		case SignalRamp:
			span := sig.Max - sig.Min
			if span <= 0 || sig.Rate <= 0 {
				v = sig.Min
			} else {
				v = sig.Min + math.Mod(sig.Rate*t, span)
			}
		case SignalRandomWalk:
			step := (rand.Float64()*2 - 1) * sig.StepMaxWalk
			v = rr.reg.Value + step
			if sig.Max > sig.Min {
				if v < sig.Min {
					v = sig.Min
				}
				if v > sig.Max {
					v = sig.Max
				}
			}
		case SignalStep:
			p := sig.Period
			if p <= 0 {
				p = 5
			}
			if math.Mod(t, p*2) < p {
				v = sig.Low
			} else {
				v = sig.High
			}
		case SignalCounter, SignalCounterRandom:
			// Handled in goroutine; just keep current value.
			v = rr.reg.Value
			rr.mu.Unlock()
			continue
		default:
			v = sig.Value
		}

		rr.reg.Value = v
		rr.reg.UpdatedAt = now.UnixMilli()
		rr.addHistory(v)
		rr.mu.Unlock()
	}
}

func (e *Engine) broadcast() {
	e.mu.RLock()
	snapshots := make([]RegisterSnapshot, 0, len(e.registers))
	for _, rr := range e.registers {
		rr.mu.Lock()
		snap := RegisterSnapshot{
			ID:        rr.reg.ID,
			Value:     rr.reg.Value,
			UpdatedAt: rr.reg.UpdatedAt,
			History:   rr.getHistory(),
		}
		rr.mu.Unlock()
		snapshots = append(snapshots, snap)
	}
	e.mu.RUnlock()

	e.subMu.RLock()
	subs := make([]chan []RegisterSnapshot, len(e.subscribers))
	copy(subs, e.subscribers)
	e.subMu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- snapshots:
		default:
			// drop if slow consumer
		}
	}
}

// Add validates and adds a register to the engine.
// Returns the assigned ID (may differ from r.ID if it was empty).
func (e *Engine) Add(r Register) (string, error) {
	if r.Name == "" {
		return "", errors.New("register name is required")
	}
	if r.DataType == "" {
		r.DataType = TypeUint16
	}
	if r.ID == "" {
		r.ID = fmt.Sprintf("reg_%d", r.Address)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.idIndex[r.ID]; exists {
		return "", fmt.Errorf("register with id %q already exists", r.ID)
	}

	// Check address conflicts.
	newAddrs := r.WordAddresses()
	for _, rr := range e.registers {
		for _, existing := range rr.reg.WordAddresses() {
			for _, na := range newAddrs {
				if existing == na {
					return "", fmt.Errorf("address %d (Modbus %d) conflicts with register %q", na, na+40001, rr.reg.ID)
				}
			}
		}
	}

	reg := r
	rr := &runtimeRegister{
		reg:       &reg,
		startTime: time.Now(),
	}

	e.registers = append(e.registers, rr)
	e.idIndex[r.ID] = rr

	if r.Signal.Kind == SignalCounter || r.Signal.Kind == SignalCounterRandom {
		e.startCounter(rr)
	}

	return r.ID, nil
}

func (e *Engine) startCounter(rr *runtimeRegister) {
	ctx, cancel := context.WithCancel(context.Background())
	rr.cancel = cancel

	go func() {
		intervalMs := rr.reg.Signal.IntervalMs
		if intervalMs <= 0 {
			intervalMs = 1000
		}
		ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rr.mu.Lock()
				sig := rr.reg.Signal
				v := rr.reg.Value
				var step float64
				if sig.Kind == SignalCounter {
					step = sig.StepMin
					if step == 0 {
						step = 1
					}
				} else {
					// counter_random
					step = rand.Float64()*(sig.StepMax-sig.StepMin) + sig.StepMin
				}
				v += step
				if sig.Max > sig.Min && v > sig.Max {
					v = sig.Min
				}
				rr.reg.Value = v
				rr.reg.UpdatedAt = time.Now().UnixMilli()
				rr.addHistory(v)
				rr.mu.Unlock()
			}
		}
	}()
}

// Update replaces a register's definition. Stops old counter goroutine if needed.
func (e *Engine) Update(id string, r Register) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	rr, exists := e.idIndex[id]
	if !exists {
		return fmt.Errorf("register %q not found", id)
	}

	// Cancel existing counter goroutine.
	if rr.cancel != nil {
		rr.cancel()
		rr.cancel = nil
	}

	// Check address conflicts (excluding self).
	newAddrs := r.WordAddresses()
	for _, other := range e.registers {
		if other.reg.ID == id {
			continue
		}
		for _, existing := range other.reg.WordAddresses() {
			for _, na := range newAddrs {
				if existing == na {
					return fmt.Errorf("address %d conflicts with register %q", na, other.reg.ID)
				}
			}
		}
	}

	r.ID = id
	reg := r

	rr.mu.Lock()
	rr.reg = &reg
	rr.startTime = time.Now()
	rr.history = [30]float64{}
	rr.historyIdx = 0
	rr.mu.Unlock()

	if r.Signal.Kind == SignalCounter || r.Signal.Kind == SignalCounterRandom {
		e.startCounter(rr)
	}

	return nil
}

// Remove removes a register by ID.
func (e *Engine) Remove(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	rr, exists := e.idIndex[id]
	if !exists {
		return fmt.Errorf("register %q not found", id)
	}

	if rr.cancel != nil {
		rr.cancel()
	}

	delete(e.idIndex, id)
	newRegs := make([]*runtimeRegister, 0, len(e.registers)-1)
	for _, r := range e.registers {
		if r.reg.ID != id {
			newRegs = append(newRegs, r)
		}
	}
	e.registers = newRegs
	return nil
}

// List returns a copy of all registers with their current values.
func (e *Engine) List() []Register {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := make([]Register, 0, len(e.registers))
	for _, rr := range e.registers {
		rr.mu.Lock()
		cp := *rr.reg
		rr.mu.Unlock()
		out = append(out, cp)
	}
	return out
}

// Get returns a single register by ID.
func (e *Engine) Get(id string) (Register, bool) {
	e.mu.RLock()
	rr, ok := e.idIndex[id]
	e.mu.RUnlock()
	if !ok {
		return Register{}, false
	}
	rr.mu.Lock()
	cp := *rr.reg
	rr.mu.Unlock()
	return cp, true
}

// Subscribe returns a channel that receives register snapshots after each tick.
func (e *Engine) Subscribe() <-chan []RegisterSnapshot {
	ch := make(chan []RegisterSnapshot, 1)
	e.subMu.Lock()
	e.subscribers = append(e.subscribers, ch)
	e.subMu.Unlock()
	return ch
}

// Unsubscribe removes a subscription channel.
func (e *Engine) Unsubscribe(ch <-chan []RegisterSnapshot) {
	e.subMu.Lock()
	defer e.subMu.Unlock()
	newSubs := make([]chan []RegisterSnapshot, 0, len(e.subscribers))
	for _, s := range e.subscribers {
		if s != ch {
			newSubs = append(newSubs, s)
		}
	}
	e.subscribers = newSubs
}

// Encode encodes the current value of register id to Modbus words.
func (e *Engine) Encode(id string) ([]uint16, error) {
	e.mu.RLock()
	rr, ok := e.idIndex[id]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("register %q not found", id)
	}
	rr.mu.Lock()
	v := rr.reg.Value
	dt := rr.reg.DataType
	rr.mu.Unlock()
	return encodeValue(v, dt), nil
}

func encodeValue(v float64, dt DataType) []uint16 {
	switch dt {
	case TypeUint16:
		return []uint16{uint16(v)}
	case TypeInt16:
		return []uint16{uint16(int16(v))}
	case TypeUint32:
		bits := uint32(v)
		return []uint16{uint16(bits & 0xFFFF), uint16(bits >> 16)}
	case TypeInt32:
		bits := uint32(int32(v))
		return []uint16{uint16(bits & 0xFFFF), uint16(bits >> 16)}
	case TypeFloat32:
		bits := math.Float32bits(float32(v))
		return []uint16{uint16(bits >> 16), uint16(bits & 0xFFFF)}
	case TypeBool:
		if v != 0 {
			return []uint16{1}
		}
		return []uint16{0}
	default:
		return []uint16{uint16(v)}
	}
}

// WordMap returns a map from each word address to the owning runtimeRegister.
func (e *Engine) WordMap() map[uint16]*runtimeRegister {
	e.mu.RLock()
	defer e.mu.RUnlock()
	m := make(map[uint16]*runtimeRegister)
	for _, rr := range e.registers {
		for _, addr := range rr.reg.WordAddresses() {
			m[addr] = rr
		}
	}
	return m
}

// WordAt implements WordReader for the Modbus server.
func (e *Engine) WordAt(addr uint16) (uint16, bool) {
	e.mu.RLock()
	var found *runtimeRegister
	for _, rr := range e.registers {
		for i, wa := range rr.reg.WordAddresses() {
			if wa == addr {
				found = rr
				_ = i
				break
			}
		}
		if found != nil {
			break
		}
	}
	e.mu.RUnlock()

	if found == nil {
		return 0, false
	}

	found.mu.Lock()
	v := found.reg.Value
	dt := found.reg.DataType
	baseAddr := found.reg.Address
	found.mu.Unlock()

	words := encodeValue(v, dt)
	wordIdx := int(addr - baseAddr)
	if wordIdx < 0 || wordIdx >= len(words) {
		return 0, false
	}
	return words[wordIdx], true
}

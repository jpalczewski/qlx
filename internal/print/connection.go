package print

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

const (
	connectTimeout = 15 * time.Second
	maxRetries     = 10
)

var backoffSteps = []time.Duration{5 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second}

type printerConn struct {
	cfg     store.PrinterConfig
	state   ConnState
	msg     string
	session *PrinterSession
	cancel  context.CancelFunc
	retries int
}

// TransportFactoryFn creates a Transport by name.
type TransportFactoryFn func(name string) transport.Transport

// EncoderLookupFn returns an Encoder by name, or nil if not found.
type EncoderLookupFn func(name string) encoder.Encoder

// ConnectionManager manages async printer connection lifecycle.
type ConnectionManager struct {
	transportFn TransportFactoryFn
	encoderFn   EncoderLookupFn

	mu       sync.RWMutex
	printers map[string]*printerConn

	sseMu      sync.Mutex
	sseClients map[chan StateChange]struct{}

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewConnectionManager creates a ConnectionManager. Call Start(ctx) before using.
func NewConnectionManager(trFn TransportFactoryFn, encFn EncoderLookupFn) *ConnectionManager {
	return &ConnectionManager{
		transportFn: trFn,
		encoderFn:   encFn,
		printers:    make(map[string]*printerConn),
		sseClients:  make(map[chan StateChange]struct{}),
	}
}

// Start initializes the manager with the given context. Non-blocking.
func (cm *ConnectionManager) Start(ctx context.Context) {
	cm.ctx, cm.cancel = context.WithCancel(ctx)
}

// Stop cancels all connections and waits for goroutines to exit.
func (cm *ConnectionManager) Stop() {
	if cm.cancel != nil {
		cm.cancel()
	}
	cm.wg.Wait()
	cm.sseMu.Lock()
	for ch := range cm.sseClients {
		close(ch)
		delete(cm.sseClients, ch)
	}
	cm.sseMu.Unlock()
}

// Add starts managing a printer — begins connecting in background.
func (cm *ConnectionManager) Add(cfg store.PrinterConfig) error {
	cm.mu.Lock()
	if _, exists := cm.printers[cfg.ID]; exists {
		cm.mu.Unlock()
		return fmt.Errorf("printer %s already managed", cfg.ID)
	}
	pCtx, pCancel := context.WithCancel(cm.ctx) //nolint:gosec // cancel stored in printerConn.cancel
	pc := &printerConn{cfg: cfg, state: StateIdle, cancel: pCancel}
	cm.printers[cfg.ID] = pc
	cm.mu.Unlock()

	cm.wg.Add(1)
	go cm.runPrinterLoop(pCtx, cfg.ID)
	return nil
}

// Remove disconnects a printer and stops managing it.
func (cm *ConnectionManager) Remove(printerID string) error {
	cm.mu.Lock()
	pc, ok := cm.printers[printerID]
	if !ok {
		cm.mu.Unlock()
		return fmt.Errorf("printer %s not managed", printerID)
	}
	pc.cancel()
	if pc.session != nil {
		pc.session.Stop()
	}
	delete(cm.printers, printerID)
	cm.mu.Unlock()
	return nil
}

// Reconnect resets backoff and retries a printer in error/any state.
func (cm *ConnectionManager) Reconnect(printerID string) error {
	cm.mu.Lock()
	pc, ok := cm.printers[printerID]
	if !ok {
		cm.mu.Unlock()
		return fmt.Errorf("printer %s not managed", printerID)
	}
	pc.cancel()
	if pc.session != nil {
		pc.session.Stop()
		pc.session = nil
	}
	pc.retries = 0
	pCtx, pCancel := context.WithCancel(cm.ctx) //nolint:gosec // cancel stored in printerConn.cancel
	pc.cancel = pCancel
	pc.state = StateIdle
	cm.mu.Unlock()

	cm.wg.Add(1)
	go cm.runPrinterLoop(pCtx, printerID)
	return nil
}

// State returns the current ConnState for a printer, or "" if unknown.
func (cm *ConnectionManager) State(printerID string) ConnState {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if pc, ok := cm.printers[printerID]; ok {
		return pc.state
	}
	return ""
}

// States returns a snapshot of all printer states.
func (cm *ConnectionManager) States() map[string]ConnState {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	out := make(map[string]ConnState, len(cm.printers))
	for id, pc := range cm.printers {
		out[id] = pc.state
	}
	return out
}

// Session returns the active PrinterSession for a printer, or nil.
func (cm *ConnectionManager) Session(printerID string) *PrinterSession {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if pc, ok := cm.printers[printerID]; ok {
		return pc.session
	}
	return nil
}

// Subscribe returns a channel of state changes. The current state of all
// printers is sent as a snapshot before live events. The snapshot is written
// atomically before the channel is registered for live events.
func (cm *ConnectionManager) Subscribe() <-chan StateChange {
	ch := make(chan StateChange, 32)
	// Hold sseMu throughout: snapshot written BEFORE channel registered.
	cm.sseMu.Lock()
	cm.mu.RLock()
	for id, pc := range cm.printers {
		ch <- StateChange{PrinterID: id, State: pc.state, Message: pc.msg, Timestamp: time.Now()}
	}
	cm.mu.RUnlock()
	cm.sseClients[ch] = struct{}{}
	cm.sseMu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (cm *ConnectionManager) Unsubscribe(ch <-chan StateChange) {
	cm.sseMu.Lock()
	defer cm.sseMu.Unlock()
	for c := range cm.sseClients {
		if c == ch {
			delete(cm.sseClients, c)
			close(c)
			break
		}
	}
}

func (cm *ConnectionManager) setState(printerID string, state ConnState, msg string) {
	cm.mu.Lock()
	pc, ok := cm.printers[printerID]
	if ok {
		pc.state = state
		pc.msg = msg
	}
	cm.mu.Unlock()
	if !ok {
		return
	}
	cm.broadcast(StateChange{PrinterID: printerID, State: state, Message: msg, Timestamp: time.Now()})
}

func (cm *ConnectionManager) broadcast(evt StateChange) {
	cm.sseMu.Lock()
	defer cm.sseMu.Unlock()
	for ch := range cm.sseClients {
		select {
		case ch <- evt:
		default:
			// slow client, drop
		}
	}
}

func (cm *ConnectionManager) runPrinterLoop(ctx context.Context, printerID string) {
	defer cm.wg.Done()
	backoffIdx := 0

	for {
		if ctx.Err() != nil {
			return
		}

		cm.setState(printerID, StateConnecting, "")

		session, err := cm.tryConnect(ctx, printerID)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			cm.mu.Lock()
			pc := cm.printers[printerID]
			if pc != nil {
				pc.retries++
				if pc.retries >= maxRetries {
					cm.mu.Unlock()
					cm.setState(printerID, StateError, err.Error())
					return
				}
			}
			cm.mu.Unlock()

			cm.setState(printerID, StateReconnecting, err.Error())
			delay := backoffSteps[backoffIdx]
			if backoffIdx < len(backoffSteps)-1 {
				backoffIdx++
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
				continue
			}
		}

		// Connected
		backoffIdx = 0
		cm.mu.Lock()
		if pc := cm.printers[printerID]; pc != nil {
			pc.session = session
			pc.retries = 0
		}
		cm.mu.Unlock()
		cm.setState(printerID, StateConnected, "")

		cm.waitForDisconnect(ctx, printerID, session)

		if ctx.Err() != nil {
			return
		}

		cm.mu.Lock()
		if pc := cm.printers[printerID]; pc != nil {
			pc.session = nil
		}
		cm.mu.Unlock()
		cm.setState(printerID, StateDisconnected, "connection lost")
	}
}

func (cm *ConnectionManager) tryConnect(ctx context.Context, printerID string) (*PrinterSession, error) {
	cm.mu.RLock()
	pc, ok := cm.printers[printerID]
	if !ok {
		cm.mu.RUnlock()
		return nil, fmt.Errorf("printer %s removed", printerID)
	}
	cfg := pc.cfg
	cm.mu.RUnlock()

	tr := cm.transportFn(cfg.Transport)
	if tr == nil {
		return nil, fmt.Errorf("unknown transport: %s", cfg.Transport)
	}
	if webutil.TraceEnabled {
		tr = &transport.TraceTransport{Inner: tr}
	}

	enc := cm.encoderFn(cfg.Encoder)
	if enc == nil {
		return nil, fmt.Errorf("unknown encoder: %s", cfg.Encoder)
	}

	// Resolve model info
	var modelInfo *encoder.ModelInfo
	for _, m := range enc.Models() {
		if m.ID == cfg.Model {
			modelInfo = &m
			break
		}
	}

	session := NewSession(cfg, tr, enc, modelInfo, nil)

	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	if err := session.Start(connectCtx); err != nil {
		return nil, err
	}
	return session, nil
}

func (cm *ConnectionManager) waitForDisconnect(ctx context.Context, _ string, session *PrinterSession) {
	select {
	case <-ctx.Done():
		session.Stop()
	case <-session.stopped:
		// heartbeat exited naturally
	}
}

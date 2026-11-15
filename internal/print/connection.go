package print

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/erxyi/qlx/internal/events"
	"github.com/erxyi/qlx/internal/print/encoder"
	"github.com/erxyi/qlx/internal/print/transport"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

const connectTimeout = 15 * time.Second

// stableConnectionThreshold is how long a session must stay up before the
// reconnect backoff is reset. A var so tests can override it.
var stableConnectionThreshold = 30 * time.Second

// backoffSteps are the reconnect delays for a printer that keeps failing or
// flapping; the last step is the cap.
var backoffSteps = []time.Duration{5 * time.Second, 10 * time.Second, 30 * time.Second}

// quickRetryDelay is the reconnect delay after a drop of a session that had
// been stable (a power cycle, a replug). A var so tests can override it.
var quickRetryDelay = time.Second

// backoff walks through backoffSteps, staying on the last step once reached.
type backoff struct{ idx int }

func (b *backoff) next() time.Duration {
	d := backoffSteps[b.idx]
	if b.idx < len(backoffSteps)-1 {
		b.idx++
	}
	return d
}

func (b *backoff) reset() { b.idx = 0 }

// sleepCtx waits for d and reports false if ctx was cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

type printerConn struct {
	cfg     store.PrinterConfig
	state   ConnState
	msg     string
	session *PrinterSession
	cancel  context.CancelFunc
	// done is closed when the runPrinterLoop goroutine for this printer exits.
	done chan struct{}
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

	events *events.EventBroker[StateChange]

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
		events:      events.NewBroker[StateChange](32),
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
	if cm.events != nil {
		cm.events.Close()
	}
}

// Add starts managing a printer — begins connecting in background.
func (cm *ConnectionManager) Add(cfg store.PrinterConfig) error {
	cm.mu.Lock()
	if _, exists := cm.printers[cfg.ID]; exists {
		cm.mu.Unlock()
		return fmt.Errorf("printer %s already managed", cfg.ID)
	}
	pCtx, pCancel := context.WithCancel(cm.ctx) //nolint:gosec // cancel stored in printerConn.cancel
	done := make(chan struct{})
	pc := &printerConn{cfg: cfg, state: StateIdle, cancel: pCancel, done: done}
	cm.printers[cfg.ID] = pc
	cm.mu.Unlock()

	cm.wg.Add(1)
	go cm.runPrinterLoop(pCtx, cfg.ID, done)
	return nil
}

// Remove disconnects a printer and stops managing it. It returns after the
// printer's connection loop has exited and its session released the transport.
func (cm *ConnectionManager) Remove(printerID string) error {
	cm.mu.Lock()
	pc, ok := cm.printers[printerID]
	if !ok {
		cm.mu.Unlock()
		return fmt.Errorf("printer %s not managed", printerID)
	}
	delete(cm.printers, printerID)
	cancel, session, done := pc.cancel, pc.session, pc.done
	cm.mu.Unlock()

	cancel()
	if session != nil {
		session.Stop()
	}
	<-done
	return nil
}

// Reconnect resets backoff and retries a printer in error/any state. The old
// connection loop is fully stopped before the new one starts, so it cannot
// still hold the USB interface while the new one opens it.
func (cm *ConnectionManager) Reconnect(printerID string) error {
	cm.mu.Lock()
	pc, ok := cm.printers[printerID]
	if !ok {
		cm.mu.Unlock()
		return fmt.Errorf("printer %s not managed", printerID)
	}
	oldCancel, oldSession, oldDone := pc.cancel, pc.session, pc.done
	pc.session = nil
	pCtx, pCancel := context.WithCancel(cm.ctx) //nolint:gosec // cancel stored in printerConn.cancel
	done := make(chan struct{})
	pc.cancel, pc.done = pCancel, done
	pc.state = StateIdle
	cm.mu.Unlock()

	oldCancel()
	if oldSession != nil {
		oldSession.Stop()
	}
	<-oldDone

	cm.wg.Add(1)
	go cm.runPrinterLoop(pCtx, printerID, done)
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

// StateInfo returns the current state and message for a printer atomically.
func (cm *ConnectionManager) StateInfo(printerID string) (ConnState, string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if pc, ok := cm.printers[printerID]; ok {
		return pc.state, pc.msg
	}
	return StateIdle, ""
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
	if cm.events == nil {
		closed := make(chan StateChange)
		close(closed)
		return closed
	}
	return cm.events.SubscribeWithSnapshot(func(out chan<- StateChange) {
		cm.mu.RLock()
		defer cm.mu.RUnlock()
		for id, pc := range cm.printers {
			out <- StateChange{PrinterID: id, State: pc.state, Message: pc.msg, Timestamp: time.Now()}
		}
	}, snapshotBufferSize(len(cm.printers)))
}

// Unsubscribe removes a subscriber and closes its channel.
func (cm *ConnectionManager) Unsubscribe(ch <-chan StateChange) {
	if cm.events != nil {
		cm.events.Unsubscribe(ch)
	}
}

func snapshotBufferSize(n int) int {
	if n < 32 {
		return 32
	}
	return n + 8
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
	if cm.events != nil {
		cm.events.Publish(StateChange{PrinterID: printerID, State: state, Message: msg, Timestamp: time.Now()})
	}
}

func (cm *ConnectionManager) runPrinterLoop(ctx context.Context, printerID string, done chan struct{}) {
	defer cm.wg.Done()
	defer close(done)
	var bo backoff

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
			cm.setState(printerID, StateReconnecting, err.Error())
			if !sleepCtx(ctx, bo.next()) {
				return
			}
			continue
		}

		// Check if we were cancelled during connect
		if ctx.Err() != nil {
			session.Stop() // clean up the just-created session
			return
		}

		// Connected — store session. The backoff is not reset here: it is reset
		// only once the session has stayed up for stableConnectionThreshold.
		connectedAt := time.Now()
		cm.mu.Lock()
		if pc := cm.printers[printerID]; pc != nil {
			pc.session = session
		}
		cm.mu.Unlock()
		cm.setState(printerID, StateConnected, "")

		cm.waitForDisconnect(ctx, session)

		if ctx.Err() != nil {
			return
		}

		reason := session.Status().LastError
		if reason == "" {
			reason = "connection lost"
		}
		stable := time.Since(connectedAt) >= stableConnectionThreshold

		cm.mu.Lock()
		if pc := cm.printers[printerID]; pc != nil {
			pc.session = nil
		}
		cm.mu.Unlock()
		cm.setState(printerID, StateDisconnected, reason)

		// A drop after a long-stable session (power cycle, replug) retries
		// quickly; a flapping printer keeps growing the delay up to the cap.
		delay := quickRetryDelay
		if stable {
			bo.reset()
		} else {
			delay = bo.next()
		}
		cm.setState(printerID, StateReconnecting, reason)
		if !sleepCtx(ctx, delay) {
			return
		}
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

// waitForDisconnect blocks until the session ends or ctx is cancelled, then
// always stops the session so its transport is released before any reconnect.
// Sessions without a heartbeat (no StatusQuerier) have a closed stopped channel
// from the start and never end by themselves, so only ctx ends them.
func (cm *ConnectionManager) waitForDisconnect(ctx context.Context, session *PrinterSession) {
	if session.querier == nil {
		<-ctx.Done()
		session.Stop()
		return
	}
	select {
	case <-ctx.Done():
	case <-session.stopped:
		// heartbeat exited on a fatal error
	}
	session.Stop()
}

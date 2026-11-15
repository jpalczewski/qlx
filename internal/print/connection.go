package print

import (
	"context"
	"errors"
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

// prevLoopWait bounds how long a new connection loop waits for the previous
// loop of the same printer to exit (and release its transport) before it
// proceeds anyway. Some Transport.Open implementations ignore their context.
var prevLoopWait = 5 * time.Second

// printerConn is one connection-loop generation for a printer. Reconnect and
// Remove+Add create a new printerConn; a loop only ever touches its own, so a
// stale generation cannot leak state into the current one.
type printerConn struct {
	cfg     store.PrinterConfig
	state   ConnState
	msg     string
	session *PrinterSession
	cancel  context.CancelFunc
	// done is closed when this generation's runPrinterLoop goroutine exits.
	done chan struct{}
	// prev is the previous generation's done channel (nil if none). The loop
	// waits for it before opening a transport, so two loops never hold the
	// same device at once.
	prev <-chan struct{}
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
	// lastDone holds the done channel of the newest loop started per printer ID,
	// including removed printers whose loop may still be releasing a transport.
	lastDone map[string]chan struct{}
	stopping bool
	// sleep waits between reconnect attempts; replaced in tests.
	sleep func(ctx context.Context, d time.Duration) bool

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
		lastDone:    make(map[string]chan struct{}),
		sleep:       sleepCtx,
		events:      events.NewBroker[StateChange](32),
	}
}

// Start initializes the manager with the given context. Non-blocking.
func (cm *ConnectionManager) Start(ctx context.Context) {
	cm.ctx, cm.cancel = context.WithCancel(ctx)
}

// Stop cancels all connections and waits for goroutines to exit.
func (cm *ConnectionManager) Stop() {
	cm.mu.Lock()
	cm.stopping = true // no new loops (and no wg.Add) after this point
	cm.mu.Unlock()
	if cm.cancel != nil {
		cm.cancel()
	}
	cm.wg.Wait()
	if cm.events != nil {
		cm.events.Close()
	}
}

// errManagerStopped is returned by Add and Reconnect once Stop was called.
var errManagerStopped = errors.New("connection manager stopped")

// startLoopLocked registers pc as the current generation for its printer and
// starts its loop. cm.mu must be held. The wg.Add happens under the lock so it
// cannot race with Stop's wg.Wait.
func (cm *ConnectionManager) startLoopLocked(pc *printerConn) {
	id := pc.cfg.ID
	pCtx, pCancel := context.WithCancel(cm.ctx) //nolint:gosec // cancel stored in printerConn.cancel
	pc.cancel = pCancel
	pc.done = make(chan struct{})
	pc.prev = cm.lastDone[id]
	cm.lastDone[id] = pc.done
	cm.printers[id] = pc

	cm.wg.Add(1)
	go cm.runPrinterLoop(pCtx, pc)
}

// Add starts managing a printer — begins connecting in background.
func (cm *ConnectionManager) Add(cfg store.PrinterConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.stopping {
		return errManagerStopped
	}
	if _, exists := cm.printers[cfg.ID]; exists {
		return fmt.Errorf("printer %s already managed", cfg.ID)
	}
	cm.startLoopLocked(&printerConn{cfg: cfg, state: StateIdle})
	return nil
}

// Remove disconnects a printer and stops managing it. It does not block: the
// loop stops its session (releasing the transport) in the background, and a
// later Add for the same ID waits for that before opening the device.
func (cm *ConnectionManager) Remove(printerID string) error {
	cm.mu.Lock()
	pc, ok := cm.printers[printerID]
	if !ok {
		cm.mu.Unlock()
		return fmt.Errorf("printer %s not managed", printerID)
	}
	delete(cm.printers, printerID)
	cm.mu.Unlock()

	pc.cancel()
	return nil
}

// Reconnect resets backoff and retries a printer in error/any state. It does
// not block: a new connection generation replaces the old one immediately and
// waits (bounded) for the old loop to release the transport before opening it.
func (cm *ConnectionManager) Reconnect(printerID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	old, ok := cm.printers[printerID]
	if !ok {
		return fmt.Errorf("printer %s not managed", printerID)
	}
	if cm.stopping {
		return errManagerStopped
	}
	cm.startLoopLocked(&printerConn{cfg: old.cfg, state: StateIdle})
	old.cancel()
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

// setState records a state on pc and publishes it, unless pc is no longer the
// current generation for its printer (removed or replaced by Reconnect).
func (cm *ConnectionManager) setState(pc *printerConn, state ConnState, msg string) {
	cm.mu.Lock()
	pc.state = state
	pc.msg = msg
	current := cm.printers[pc.cfg.ID] == pc
	cm.mu.Unlock()
	if !current {
		return
	}
	if cm.events != nil {
		cm.events.Publish(StateChange{PrinterID: pc.cfg.ID, State: state, Message: msg, Timestamp: time.Now()})
	}
}

// waitPrev waits (bounded) for the previous generation's loop to exit. It
// returns false if ctx was cancelled first.
func (cm *ConnectionManager) waitPrev(ctx context.Context, pc *printerConn) bool {
	if pc.prev == nil {
		return true
	}
	timer := time.NewTimer(prevLoopWait)
	defer timer.Stop()
	select {
	case <-pc.prev:
	case <-timer.C:
		webutil.LogError("printer %s: previous connection loop did not exit in %v; continuing", pc.cfg.Name, prevLoopWait)
	case <-ctx.Done():
		return false
	}
	return true
}

func (cm *ConnectionManager) runPrinterLoop(ctx context.Context, pc *printerConn) {
	defer cm.wg.Done()
	defer close(pc.done)
	if !cm.waitPrev(ctx, pc) {
		return
	}
	var bo backoff

	for ctx.Err() == nil {
		cm.setState(pc, StateConnecting, "")

		session, err := cm.tryConnect(ctx, pc.cfg)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			cm.setState(pc, StateReconnecting, err.Error())
			if !cm.sleep(ctx, bo.next()) {
				return
			}
			continue
		}

		// Check if we were cancelled during connect
		if ctx.Err() != nil {
			session.Stop() // clean up the just-created session
			return
		}

		reason, stable := cm.runSession(ctx, pc, session)
		if ctx.Err() != nil {
			return
		}

		// A drop after a long-stable session (power cycle, replug) retries
		// quickly; a flapping printer keeps growing the delay up to the cap.
		// There is no hysteresis: one stable session resets the backoff.
		delay := quickRetryDelay
		if stable {
			bo.reset()
		} else {
			delay = bo.next()
		}
		cm.setState(pc, StateReconnecting, reason)
		if !cm.sleep(ctx, delay) {
			return
		}
	}
}

// runSession publishes the connected session, waits until it ends and stops it
// so the transport is released. It returns the reason the session ended and
// whether it had been up for stableConnectionThreshold. The backoff is reset
// only for stable sessions.
func (cm *ConnectionManager) runSession(ctx context.Context, pc *printerConn, session *PrinterSession) (reason string, stable bool) {
	connectedAt := time.Now()
	cm.mu.Lock()
	pc.session = session
	cm.mu.Unlock()
	cm.setState(pc, StateConnected, "")

	cm.waitForDisconnect(ctx, session)

	reason = session.Status().LastError
	if reason == "" {
		reason = "connection lost"
	}
	stable = time.Since(connectedAt) >= stableConnectionThreshold

	cm.mu.Lock()
	pc.session = nil
	cm.mu.Unlock()
	if ctx.Err() == nil {
		cm.setState(pc, StateDisconnected, reason)
	}
	return reason, stable
}

func (cm *ConnectionManager) tryConnect(ctx context.Context, cfg store.PrinterConfig) (*PrinterSession, error) {
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

// waitForDisconnect blocks until the session fails or ctx is cancelled, then
// always stops the session so its transport is released before any reconnect.
// A session fails on a fatal heartbeat error or, for encoders without a
// heartbeat, on a transport error while printing.
func (cm *ConnectionManager) waitForDisconnect(ctx context.Context, session *PrinterSession) {
	select {
	case <-ctx.Done():
	case <-session.failed:
	}
	session.Stop()
}

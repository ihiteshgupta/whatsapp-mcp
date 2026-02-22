package state

import (
	"context"
	"sync"

	"github.com/qmuntal/stateless"
)

// TransitionCallback is called when a state transition occurs.
type TransitionCallback func(ctx context.Context, from, to State, trigger Trigger)

// Machine wraps the stateless state machine with WhatsApp bridge-specific behavior.
type Machine struct {
	sm          *stateless.StateMachine
	callbacks   []TransitionCallback
	callbacksMu sync.RWMutex
}

// NewMachine creates a new state machine starting in Disconnected state.
func NewMachine() *Machine {
	m := &Machine{
		callbacks: make([]TransitionCallback, 0),
	}

	sm := stateless.NewStateMachine(StateDisconnected)

	// Configure Disconnected state
	sm.Configure(StateDisconnected).
		Permit(TriggerConnect, StateConnecting).
		Permit(TriggerShutdown, StateShuttingDown)

	// Configure Connecting state
	sm.Configure(StateConnecting).
		Permit(TriggerQRRequired, StateQRPending).
		Permit(TriggerAuthenticated, StateSyncing).
		Permit(TriggerConnectionLost, StateReconnecting).
		Permit(TriggerDisconnect, StateDisconnected).
		Permit(TriggerShutdown, StateShuttingDown).
		Permit(TriggerFatalError, StateFatalError)

	// Configure QRPending state (substate of Connected conceptually)
	sm.Configure(StateQRPending).
		Permit(TriggerQRScanned, StateAuthenticating).
		Permit(TriggerConnectionLost, StateReconnecting).
		Permit(TriggerDisconnect, StateDisconnected).
		Permit(TriggerShutdown, StateShuttingDown).
		Permit(TriggerFatalError, StateFatalError)

	// Configure Authenticating state
	sm.Configure(StateAuthenticating).
		Permit(TriggerAuthenticated, StateSyncing).
		Permit(TriggerSessionInvalid, StateSessionExpired).
		Permit(TriggerConnectionLost, StateReconnecting).
		Permit(TriggerDisconnect, StateDisconnected).
		Permit(TriggerShutdown, StateShuttingDown).
		Permit(TriggerFatalError, StateFatalError)

	// Configure Syncing state
	sm.Configure(StateSyncing).
		Permit(TriggerSyncComplete, StateReady).
		Permit(TriggerConnectionLost, StateReconnecting).
		Permit(TriggerDisconnect, StateDisconnected).
		Permit(TriggerShutdown, StateShuttingDown).
		Permit(TriggerFatalError, StateFatalError)

	// Configure Ready state (fully operational)
	sm.Configure(StateReady).
		Permit(TriggerConnectionLost, StateReconnecting).
		Permit(TriggerSessionInvalid, StateSessionExpired).
		Permit(TriggerBanDetected, StateTemporaryBan).
		Permit(TriggerLogout, StateLoggedOut).
		Permit(TriggerDisconnect, StateDisconnected).
		Permit(TriggerShutdown, StateShuttingDown).
		Permit(TriggerFatalError, StateFatalError)

	// Configure Reconnecting state
	sm.Configure(StateReconnecting).
		Permit(TriggerReconnected, StateReady).
		Permit(TriggerQRRequired, StateQRPending).
		Permit(TriggerSessionInvalid, StateSessionExpired).
		Permit(TriggerDisconnect, StateDisconnected).
		Permit(TriggerShutdown, StateShuttingDown).
		Permit(TriggerFatalError, StateFatalError)

	// Configure TemporaryBan state
	sm.Configure(StateTemporaryBan).
		Permit(TriggerBanLifted, StateReconnecting).
		Permit(TriggerDisconnect, StateDisconnected).
		Permit(TriggerShutdown, StateShuttingDown).
		Permit(TriggerFatalError, StateFatalError)

	// Configure terminal states (limited transitions out)
	sm.Configure(StateLoggedOut).
		Permit(TriggerConnect, StateConnecting).
		Permit(TriggerShutdown, StateShuttingDown)

	sm.Configure(StateSessionExpired).
		Permit(TriggerConnect, StateConnecting).
		Permit(TriggerShutdown, StateShuttingDown)

	sm.Configure(StateShuttingDown)
	// No transitions out of ShuttingDown

	sm.Configure(StateFatalError).
		Permit(TriggerShutdown, StateShuttingDown)

	// Set up transition callback
	sm.OnTransitioned(func(ctx context.Context, t stateless.Transition) {
		m.callbacksMu.RLock()
		callbacks := make([]TransitionCallback, len(m.callbacks))
		copy(callbacks, m.callbacks)
		m.callbacksMu.RUnlock()

		from := t.Source.(State)
		to := t.Destination.(State)
		trigger := t.Trigger.(Trigger)

		for _, cb := range callbacks {
			cb(ctx, from, to, trigger)
		}
	})

	m.sm = sm
	return m
}

// State returns the current state.
func (m *Machine) State(ctx context.Context) (State, error) {
	state, err := m.sm.State(ctx)
	if err != nil {
		return "", err
	}
	return state.(State), nil
}

// Fire triggers a state transition.
func (m *Machine) Fire(ctx context.Context, trigger Trigger, args ...any) error {
	return m.sm.FireCtx(ctx, trigger, args...)
}

// CanFire returns true if the trigger can be fired from the current state.
func (m *Machine) CanFire(ctx context.Context, trigger Trigger, args ...any) (bool, error) {
	return m.sm.CanFireCtx(ctx, trigger, args...)
}

// IsInState returns true if the machine is in the specified state.
func (m *Machine) IsInState(ctx context.Context, state State) (bool, error) {
	currentState, err := m.State(ctx)
	if err != nil {
		return false, err
	}
	return currentState == state, nil
}

// OnTransition registers a callback to be called on state transitions.
func (m *Machine) OnTransition(cb TransitionCallback) {
	m.callbacksMu.Lock()
	defer m.callbacksMu.Unlock()
	m.callbacks = append(m.callbacks, cb)
}

// MustState returns the current state, panicking on error.
func (m *Machine) MustState() State {
	state, err := m.State(context.Background())
	if err != nil {
		panic(err)
	}
	return state
}

// IsReady returns true if the bridge is in Ready state.
func (m *Machine) IsReady() bool {
	state := m.MustState()
	return state == StateReady
}

// IsConnected returns true if the bridge is in a connected substate.
func (m *Machine) IsConnected() bool {
	state := m.MustState()
	return state.IsConnectedSubstate() || state == StateConnecting
}

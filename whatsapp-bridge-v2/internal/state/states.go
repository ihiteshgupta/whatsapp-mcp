// Package state provides the finite state machine for WhatsApp bridge connection lifecycle.
package state

// State represents a connection state in the bridge lifecycle.
type State string

const (
	// Primary states
	StateDisconnected  State = "disconnected"
	StateConnecting    State = "connecting"
	StateConnected     State = "connected"
	StateReconnecting  State = "reconnecting"

	// Substates of Connected
	StateQRPending      State = "qr_pending"
	StateAuthenticating State = "authenticating"
	StateSyncing        State = "syncing"
	StateReady          State = "ready"

	// Terminal/Error states
	StateLoggedOut      State = "logged_out"
	StateSessionExpired State = "session_expired"
	StateTemporaryBan   State = "temporary_ban"
	StateShuttingDown   State = "shutting_down"
	StateFatalError     State = "fatal_error"
)

// String returns the string representation of the state.
func (s State) String() string {
	return string(s)
}

// IsConnectedSubstate returns true if the state is a substate of Connected.
func (s State) IsConnectedSubstate() bool {
	switch s {
	case StateQRPending, StateAuthenticating, StateSyncing, StateReady:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the state is a terminal/error state.
func (s State) IsTerminal() bool {
	switch s {
	case StateLoggedOut, StateSessionExpired, StateTemporaryBan, StateFatalError:
		return true
	default:
		return false
	}
}

// IsOperational returns true if the bridge can perform operations.
func (s State) IsOperational() bool {
	return s == StateReady
}

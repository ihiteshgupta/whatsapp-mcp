package state

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMachine(t *testing.T) {
	m := NewMachine()
	require.NotNil(t, m)

	state, err := m.State(context.Background())
	require.NoError(t, err)
	assert.Equal(t, StateDisconnected, state)
}

func TestMachine_ConnectFlow(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Start in disconnected
	state, _ := m.State(ctx)
	assert.Equal(t, StateDisconnected, state)

	// Connect -> Connecting
	err := m.Fire(ctx, TriggerConnect)
	require.NoError(t, err)
	state, _ = m.State(ctx)
	assert.Equal(t, StateConnecting, state)
}

func TestMachine_QRFlow(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Connect
	_ = m.Fire(ctx, TriggerConnect)

	// QR Required -> QRPending
	err := m.Fire(ctx, TriggerQRRequired)
	require.NoError(t, err)
	state, _ := m.State(ctx)
	assert.Equal(t, StateQRPending, state)

	// QR Scanned -> Authenticating
	err = m.Fire(ctx, TriggerQRScanned)
	require.NoError(t, err)
	state, _ = m.State(ctx)
	assert.Equal(t, StateAuthenticating, state)

	// Authenticated -> Syncing
	err = m.Fire(ctx, TriggerAuthenticated)
	require.NoError(t, err)
	state, _ = m.State(ctx)
	assert.Equal(t, StateSyncing, state)

	// Sync Complete -> Ready
	err = m.Fire(ctx, TriggerSyncComplete)
	require.NoError(t, err)
	state, _ = m.State(ctx)
	assert.Equal(t, StateReady, state)
}

func TestMachine_DirectAuthFlow(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Connect
	_ = m.Fire(ctx, TriggerConnect)

	// Direct authentication (existing session)
	err := m.Fire(ctx, TriggerAuthenticated)
	require.NoError(t, err)
	state, _ := m.State(ctx)
	assert.Equal(t, StateSyncing, state)

	// Sync Complete -> Ready
	err = m.Fire(ctx, TriggerSyncComplete)
	require.NoError(t, err)
	state, _ = m.State(ctx)
	assert.Equal(t, StateReady, state)
}

func TestMachine_ReconnectionFlow(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Get to Ready state
	_ = m.Fire(ctx, TriggerConnect)
	_ = m.Fire(ctx, TriggerAuthenticated)
	_ = m.Fire(ctx, TriggerSyncComplete)

	state, _ := m.State(ctx)
	assert.Equal(t, StateReady, state)

	// Connection lost -> Reconnecting
	err := m.Fire(ctx, TriggerConnectionLost)
	require.NoError(t, err)
	state, _ = m.State(ctx)
	assert.Equal(t, StateReconnecting, state)

	// Reconnected -> Ready
	err = m.Fire(ctx, TriggerReconnected)
	require.NoError(t, err)
	state, _ = m.State(ctx)
	assert.Equal(t, StateReady, state)
}

func TestMachine_DisconnectFromReady(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Get to Ready state
	_ = m.Fire(ctx, TriggerConnect)
	_ = m.Fire(ctx, TriggerAuthenticated)
	_ = m.Fire(ctx, TriggerSyncComplete)

	// Disconnect -> Disconnected
	err := m.Fire(ctx, TriggerDisconnect)
	require.NoError(t, err)
	state, _ := m.State(ctx)
	assert.Equal(t, StateDisconnected, state)
}

func TestMachine_SessionExpired(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Get to Ready state
	_ = m.Fire(ctx, TriggerConnect)
	_ = m.Fire(ctx, TriggerAuthenticated)
	_ = m.Fire(ctx, TriggerSyncComplete)

	// Session Invalid -> SessionExpired
	err := m.Fire(ctx, TriggerSessionInvalid)
	require.NoError(t, err)
	state, _ := m.State(ctx)
	assert.Equal(t, StateSessionExpired, state)
}

func TestMachine_TemporaryBan(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Get to Ready state
	_ = m.Fire(ctx, TriggerConnect)
	_ = m.Fire(ctx, TriggerAuthenticated)
	_ = m.Fire(ctx, TriggerSyncComplete)

	// Ban detected -> TemporaryBan
	err := m.Fire(ctx, TriggerBanDetected)
	require.NoError(t, err)
	state, _ := m.State(ctx)
	assert.Equal(t, StateTemporaryBan, state)

	// Ban lifted -> Reconnecting
	err = m.Fire(ctx, TriggerBanLifted)
	require.NoError(t, err)
	state, _ = m.State(ctx)
	assert.Equal(t, StateReconnecting, state)
}

func TestMachine_Logout(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Get to Ready state
	_ = m.Fire(ctx, TriggerConnect)
	_ = m.Fire(ctx, TriggerAuthenticated)
	_ = m.Fire(ctx, TriggerSyncComplete)

	// Logout -> LoggedOut
	err := m.Fire(ctx, TriggerLogout)
	require.NoError(t, err)
	state, _ := m.State(ctx)
	assert.Equal(t, StateLoggedOut, state)
}

func TestMachine_Shutdown(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Get to Ready state
	_ = m.Fire(ctx, TriggerConnect)
	_ = m.Fire(ctx, TriggerAuthenticated)
	_ = m.Fire(ctx, TriggerSyncComplete)

	// Shutdown -> ShuttingDown
	err := m.Fire(ctx, TriggerShutdown)
	require.NoError(t, err)
	state, _ := m.State(ctx)
	assert.Equal(t, StateShuttingDown, state)
}

func TestMachine_FatalError(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Get to Ready state
	_ = m.Fire(ctx, TriggerConnect)
	_ = m.Fire(ctx, TriggerAuthenticated)
	_ = m.Fire(ctx, TriggerSyncComplete)

	// Fatal error -> FatalError
	err := m.Fire(ctx, TriggerFatalError)
	require.NoError(t, err)
	state, _ := m.State(ctx)
	assert.Equal(t, StateFatalError, state)
}

func TestMachine_InvalidTransition(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Try to sync complete from disconnected (invalid)
	err := m.Fire(ctx, TriggerSyncComplete)
	assert.Error(t, err)
}

func TestMachine_IsInState(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Get to Ready state
	_ = m.Fire(ctx, TriggerConnect)
	_ = m.Fire(ctx, TriggerAuthenticated)
	_ = m.Fire(ctx, TriggerSyncComplete)

	isReady, err := m.IsInState(ctx, StateReady)
	require.NoError(t, err)
	assert.True(t, isReady)

	isDisconnected, err := m.IsInState(ctx, StateDisconnected)
	require.NoError(t, err)
	assert.False(t, isDisconnected)
}

func TestMachine_CanFire(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// From disconnected, can fire Connect
	canConnect, err := m.CanFire(ctx, TriggerConnect)
	require.NoError(t, err)
	assert.True(t, canConnect)

	// From disconnected, cannot fire SyncComplete
	canSync, err := m.CanFire(ctx, TriggerSyncComplete)
	require.NoError(t, err)
	assert.False(t, canSync)
}

func TestMachine_ConnectionLostFromSyncing(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	// Get to Syncing state
	_ = m.Fire(ctx, TriggerConnect)
	_ = m.Fire(ctx, TriggerAuthenticated)

	state, _ := m.State(ctx)
	assert.Equal(t, StateSyncing, state)

	// Connection lost from Syncing -> Reconnecting
	err := m.Fire(ctx, TriggerConnectionLost)
	require.NoError(t, err)
	state, _ = m.State(ctx)
	assert.Equal(t, StateReconnecting, state)
}

func TestMachine_ShutdownFromAnyState(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(m *Machine)
		fromState  State
	}{
		{
			name:      "from disconnected",
			setupFunc: func(m *Machine) {},
			fromState: StateDisconnected,
		},
		{
			name: "from connecting",
			setupFunc: func(m *Machine) {
				_ = m.Fire(context.Background(), TriggerConnect)
			},
			fromState: StateConnecting,
		},
		{
			name: "from ready",
			setupFunc: func(m *Machine) {
				ctx := context.Background()
				_ = m.Fire(ctx, TriggerConnect)
				_ = m.Fire(ctx, TriggerAuthenticated)
				_ = m.Fire(ctx, TriggerSyncComplete)
			},
			fromState: StateReady,
		},
		{
			name: "from reconnecting",
			setupFunc: func(m *Machine) {
				ctx := context.Background()
				_ = m.Fire(ctx, TriggerConnect)
				_ = m.Fire(ctx, TriggerAuthenticated)
				_ = m.Fire(ctx, TriggerSyncComplete)
				_ = m.Fire(ctx, TriggerConnectionLost)
			},
			fromState: StateReconnecting,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := NewMachine()
			tt.setupFunc(m)

			state, _ := m.State(ctx)
			assert.Equal(t, tt.fromState, state)

			err := m.Fire(ctx, TriggerShutdown)
			require.NoError(t, err)

			state, _ = m.State(ctx)
			assert.Equal(t, StateShuttingDown, state)
		})
	}
}

func TestMachine_OnTransitionCallback(t *testing.T) {
	ctx := context.Background()
	m := NewMachine()

	var transitions []struct {
		from    State
		to      State
		trigger Trigger
	}

	m.OnTransition(func(ctx context.Context, from, to State, trigger Trigger) {
		transitions = append(transitions, struct {
			from    State
			to      State
			trigger Trigger
		}{from, to, trigger})
	})

	// Perform some transitions
	_ = m.Fire(ctx, TriggerConnect)
	_ = m.Fire(ctx, TriggerAuthenticated)
	_ = m.Fire(ctx, TriggerSyncComplete)

	assert.Len(t, transitions, 3)
	assert.Equal(t, StateDisconnected, transitions[0].from)
	assert.Equal(t, StateConnecting, transitions[0].to)
	assert.Equal(t, TriggerConnect, transitions[0].trigger)
}

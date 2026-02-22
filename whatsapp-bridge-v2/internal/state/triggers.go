package state

// Trigger represents an event that causes a state transition.
type Trigger string

const (
	TriggerConnect        Trigger = "connect"
	TriggerDisconnect     Trigger = "disconnect"
	TriggerQRRequired     Trigger = "qr_required"
	TriggerQRScanned      Trigger = "qr_scanned"
	TriggerAuthenticated  Trigger = "authenticated"
	TriggerSyncComplete   Trigger = "sync_complete"
	TriggerConnectionLost Trigger = "connection_lost"
	TriggerReconnect      Trigger = "reconnect"
	TriggerReconnected    Trigger = "reconnected"
	TriggerSessionInvalid Trigger = "session_invalid"
	TriggerBanDetected    Trigger = "ban_detected"
	TriggerBanLifted      Trigger = "ban_lifted"
	TriggerLogout         Trigger = "logout"
	TriggerShutdown       Trigger = "shutdown"
	TriggerFatalError     Trigger = "fatal_error"
)

// String returns the string representation of the trigger.
func (t Trigger) String() string {
	return string(t)
}

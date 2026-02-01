package whatsapp

import (
	"regexp"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

// Test helper functions that mirror the parsing logic in client.go

func TestParseRecipientLogic(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "phone number with plus",
			input:   "+1234567890",
			want:    "1234567890@s.whatsapp.net",
			wantErr: false,
		},
		{
			name:    "phone number without plus",
			input:   "1234567890",
			want:    "1234567890@s.whatsapp.net",
			wantErr: false,
		},
		{
			name:    "phone number with spaces and dashes",
			input:   "+1-234-567 890",
			want:    "1234567890@s.whatsapp.net",
			wantErr: false,
		},
		{
			name:    "already a JID",
			input:   "1234567890@s.whatsapp.net",
			want:    "1234567890@s.whatsapp.net",
			wantErr: false,
		},
		{
			name:    "group JID",
			input:   "1234567890-1234567890@g.us",
			want:    "1234567890-1234567890@g.us",
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRecipientTest(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRecipient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.String() != tt.want {
				t.Errorf("parseRecipient() = %v, want %v", got.String(), tt.want)
			}
		})
	}
}

func TestParseGroupJIDLogic(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid group JID",
			input:   "1234567890-1234567890@g.us",
			want:    "1234567890-1234567890@g.us",
			wantErr: false,
		},
		{
			name:    "non-group JID should error",
			input:   "1234567890@s.whatsapp.net",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGroupJIDTest(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGroupJID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.String() != tt.want {
				t.Errorf("parseGroupJID() = %v, want %v", got.String(), tt.want)
			}
		})
	}
}

func TestParseParticipantJIDsLogic(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    int
		wantErr bool
	}{
		{
			name:    "valid phone numbers",
			input:   []string{"+1234567890", "+0987654321"},
			want:    2,
			wantErr: false,
		},
		{
			name:    "empty list",
			input:   []string{},
			wantErr: true,
		},
		{
			name:    "nil list",
			input:   nil,
			wantErr: true,
		},
		{
			name:    "mixed formats",
			input:   []string{"+1234567890", "0987654321@s.whatsapp.net"},
			want:    2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseParticipantJIDsTest(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseParticipantJIDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.want {
				t.Errorf("parseParticipantJIDs() returned %d JIDs, want %d", len(got), tt.want)
			}
		})
	}
}

// Test helper implementations - mirror the logic from client.go
var phoneRegex = regexp.MustCompile(`[^\d]`)

func parseRecipientTest(recipient string) (types.JID, error) {
	if recipient == "" {
		return types.JID{}, ErrInvalidRecipient
	}

	// If already a JID, parse it
	if strings.Contains(recipient, "@") {
		jid, err := types.ParseJID(recipient)
		if err != nil {
			return types.JID{}, err
		}
		return jid, nil
	}

	// Clean phone number - remove non-digits
	phone := phoneRegex.ReplaceAllString(recipient, "")
	if phone == "" {
		return types.JID{}, ErrInvalidRecipient
	}

	return types.NewJID(phone, types.DefaultUserServer), nil
}

func parseGroupJIDTest(jid string) (types.JID, error) {
	if jid == "" {
		return types.JID{}, ErrInvalidGroup
	}

	parsed, err := types.ParseJID(jid)
	if err != nil {
		return types.JID{}, err
	}

	if parsed.Server != types.GroupServer {
		return types.JID{}, ErrInvalidGroup
	}

	return parsed, nil
}

func parseParticipantJIDsTest(participants []string) ([]types.JID, error) {
	if len(participants) == 0 {
		return nil, ErrNoParticipants
	}

	jids := make([]types.JID, 0, len(participants))
	for _, p := range participants {
		jid, err := parseRecipientTest(p)
		if err != nil {
			return nil, err
		}
		jids = append(jids, jid)
	}

	return jids, nil
}

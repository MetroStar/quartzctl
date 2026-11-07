package privateaccess

import "testing"

func TestSessionMetadataRetainsValidIdentifierPrefixes(t *testing.T) {
	for _, id := range []string{"_operator-session", "-operator-session", "operator-session"} {
		metadata := &sessionMetadata{}
		metadata.Write([]byte("Starting session with SessionId: " + id + "\n"))
		if !metadata.ready() || metadata.id != id {
			t.Fatal("valid session identifier prefix was lost", id)
		}
	}
}

func TestSessionMetadataRequiresCompleteIdentityLine(t *testing.T) {
	metadata := &sessionMetadata{}
	metadata.Write([]byte("Starting session with SessionId: fixture-pa"))
	if metadata.ready() {
		t.Fatal("partial session identity must not be used for termination")
	}
	metadata.Write([]byte("rt\r\n"))
	if !metadata.ready() || metadata.id != "fixture-part" || metadata.buffer != "" {
		t.Fatal("session identity must be captured without retaining diagnostic output")
	}
}

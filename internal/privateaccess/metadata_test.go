package privateaccess

import "testing"

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

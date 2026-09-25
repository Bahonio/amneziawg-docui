package agentclient

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestUnavailableSocketHasOperatorFacingError(t *testing.T) {
	client := New(filepath.Join(t.TempDir(), "missing.sock"))
	_, err := client.Health()
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v", err)
	}
}

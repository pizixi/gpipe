//go:build windows

package upgrade

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestAtomicReplaceClearsReadOnlyDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "candidate.exe")
	destination := filepath.Join(dir, "client.exe")
	if err := os.WriteFile(source, []byte("new-client"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old-client"), 0o600); err != nil {
		t.Fatal(err)
	}
	destinationPtr, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		t.Fatal(err)
	}
	attributes, err := windows.GetFileAttributes(destinationPtr)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetFileAttributes(destinationPtr, attributes|windows.FILE_ATTRIBUTE_READONLY); err != nil {
		t.Fatal(err)
	}
	if err := atomicReplace(source, destination); err != nil {
		t.Fatalf("replace read-only executable: %v", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-client" {
		t.Fatalf("destination = %q, want new client", data)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists after replacement: %v", err)
	}
}

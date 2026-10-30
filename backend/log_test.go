package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/packetflinger/q2admind/frontend"
)

func TestFrontendLogPath(t *testing.T) {
	tests := []struct {
		name, uuid string
		want       string
	}{
		{"dm", "1234-abcd", filepath.Join("log", "dm-1234-abcd.log")},
		{"", "1234", ""},
		{"dm", "", ""},
		{"../dm", "1234", ""},
		{"dm", "a/b", ""},
	}
	for _, tc := range tests {
		got, err := FrontendLogPath(&frontend.Frontend{Name: tc.name, UUID: tc.uuid})
		if tc.want == "" {
			if err == nil {
				t.Errorf("FrontendLogPath(%q, %q) = %q, want an error", tc.name, tc.uuid, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("FrontendLogPath(%q, %q) = %q, %v, want %q", tc.name, tc.uuid, got, err, tc.want)
		}
	}
}

// The log directory gets created if it's missing, and reconnects reuse the
// open file instead of leaking a new one each time.
func TestNewFrontendLogger(t *testing.T) {
	t.Chdir(t.TempDir())

	fe := &frontend.Frontend{Name: "dm", UUID: "1234"}
	l, err := NewFrontendLogger(fe)
	if err != nil {
		t.Fatal(err)
	}
	fe.Log = l
	l.Println("connecting")
	first := fe.LogFile

	l2, err := NewFrontendLogger(fe)
	if err != nil {
		t.Fatal(err)
	}
	if l2 != l || fe.LogFile != first {
		t.Error("second call opened a new file")
	}
	fe.LogFile.Close()

	data, err := os.ReadFile(filepath.Join("log", "dm-1234.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "connecting") {
		t.Errorf("log file = %q, want it to contain %q", data, "connecting")
	}

	if l, err := NewFrontendLogger(&frontend.Frontend{Name: "x/y", UUID: "1"}); err == nil || l == nil {
		t.Errorf("bad name: got logger %v, err %v; want a fallback logger and an error", l, err)
	}
}

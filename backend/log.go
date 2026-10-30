package backend

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/packetflinger/q2admind/frontend"
)

// FrontendLogDirectory holds every frontend's log file. It's created if it
// doesn't exist.
const FrontendLogDirectory = "log"

// FrontendLogPath is the log file for a frontend: log/<name>-<uuid>.log.
func FrontendLogPath(fe *frontend.Frontend) (string, error) {
	if fe == nil {
		return "", fmt.Errorf("null frontend")
	}
	base := fe.Name + "-" + fe.UUID
	if fe.Name == "" || fe.UUID == "" || strings.ContainsAny(base, `/\`) || strings.Contains(base, "..") {
		return "", fmt.Errorf("can't make a log file name from name %q and id %q", fe.Name, fe.UUID)
	}
	return filepath.Join(FrontendLogDirectory, base+".log"), nil
}

// NewFrontendLogger returns a logger writing to the frontend's log file (see
// FrontendLogPath). The file stays open across reconnects, so calling this
// again for the same frontend reuses it.
//
// It always returns a usable logger. If the file can't be opened, the error
// is returned along with a logger that writes to stderr instead.
func NewFrontendLogger(fe *frontend.Frontend) (*log.Logger, error) {
	flags := log.Ldate | log.Ltime | log.Lmicroseconds
	if fe == nil {
		return log.New(os.Stderr, "", flags), fmt.Errorf("null frontend")
	}
	fallback := log.New(os.Stderr, "["+fe.Name+"] ", flags)
	logfile, err := FrontendLogPath(fe)
	if err != nil {
		return fallback, err
	}
	if fe.LogFile != nil && fe.Log != nil && fe.LogFile.Name() == logfile {
		return fe.Log, nil
	}
	if err := os.MkdirAll(FrontendLogDirectory, 0700); err != nil {
		return fallback, err
	}
	fp, err := os.OpenFile(logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fallback, err
	}
	if fe.LogFile != nil {
		fe.LogFile.Close()
	}
	fe.LogFile = fp
	return log.New(fp, "", flags), nil
}

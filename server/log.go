package server

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/packetflinger/q2admind/frontend"
)

// Each client has it's own logger and dedicated log file along side
// the client's other files. This log is generally just lines of raw
// text.
//
// Open the file and return a logger object for it.
func NewFrontendLogger(fe *frontend.Frontend) (*log.Logger, error) {
	if fe == nil {
		return nil, fmt.Errorf("null client")
	}
	logfile := path.Join(srv.config.ClientDirectory, fe.Name, "log")
	fp, err := os.OpenFile(logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	fe.LogFile = fp
	flags := log.Ldate | log.Ltime | log.Lmicroseconds
	return log.New(fp, "", flags), nil
}

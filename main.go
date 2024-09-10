package main

import (
	"flag"
	"os"
	"os/signal"

	"github.com/packetflinger/q2admind/server"
)

var (
	config     = flag.String("config", "config/config.pb", "The main config file")
	foreground = flag.Bool("foreground", false, "log to the console or file")
)

// start here
func main() {
	flag.Parse()

	// catch stuff like ctrl+c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		server.Shutdown()
		os.Exit(0)
	}()

	// run it
	server.Startup(*config, *foreground)
}

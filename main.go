// CloudAdmin is a centralized management service for Quake 2 game servers
// running the q2admin game library. The game library will make a persistent
// TCP connection to this service for logging and player management.
package main

import (
	"flag"
	"os"
	"os/signal"

	"github.com/packetflinger/q2admind/server"
)

var (
	config     = flag.String("config", "config/config.pb", "The main config file")
	foreground = flag.Bool("foreground", false, "Log to the console or file")
)

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

	server.Startup(*config, *foreground)
}

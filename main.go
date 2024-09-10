package main

import (
	"flag"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/packetflinger/q2admind/server"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	config     = flag.String("config", "config/config.pb", "The main config file")
	foreground = flag.Bool("foreground", false, "log to the console or file")
)

// start here
func main() {
	flag.Parse()

	log.Println("Loading config:", *config)
	textpb, err := os.ReadFile(*config)
	if err != nil {
		log.Fatal(err)
	}

	err = prototext.Unmarshal(textpb, &server.Cloud.Config)
	if err != nil {
		log.Fatal(err)
	}

	server.Cloud.Config.Foreground = *foreground

	// not needed in Go 1.20+
	rand.Seed(time.Now().Unix())

	// catch stuff like ctrl+c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		server.Shutdown()
		os.Exit(0)
	}()

	// run it
	server.Startup()
}

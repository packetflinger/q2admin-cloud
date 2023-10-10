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
	Configfile = flag.String("config", "config/config", "The main config file")
)

// start here
func main() {
	flag.Parse()

	log.Println("Loading config:", *Configfile)
	textpb, err := os.ReadFile(*Configfile)
	if err != nil {
		log.Fatal(err)
	}

	err = prototext.Unmarshal(textpb, &server.Q2A.Config)
	if err != nil {
		log.Fatal(err)
	}

	// not needed in Go 1.20+
	rand.Seed(time.Now().Unix())

	// catch stuff like ctrl+c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		server.Shutdown()
		os.Exit(1)
	}()

	// run it
	server.Startup()
}

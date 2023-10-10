package main

import (
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/packetflinger/q2admind/api"
	"github.com/packetflinger/q2admind/server"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	Configfile = flag.String("config", "config/config", "The main config file")
)

// start here
func main() {
	initialize()
	// catch stuff like ctrl+c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		server.Shutdown()
		os.Exit(1)
	}()

	port := fmt.Sprintf("%s:%d", server.Q2A.config.Address, server.Q2A.config.Port)
	listener, err := net.Listen("tcp", port) // v4 + v6
	if err != nil {
		fmt.Println(err)
		return
	}

	defer listener.Close()

	log.Printf("Listening for gameservers on %s\n", port)

	if server.Q2A.config.GetApiEnabled() {
		go api.RunHTTPServer()
	}

	go server.Q2A.Maintenance()

	for {
		c, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		go server.HandleConnection(c)
	}
}

// This is a renamed "init()" function. Having it named init
// was messing up the unit tests.
//
// Called from main() at startup
func initialize() {
	flag.Parse()

	log.Println("Loading config:", *Configfile)
	textpb, err := os.ReadFile(*Configfile)
	if err != nil {
		log.Fatal(err)
	}

	err = prototext.Unmarshal(textpb, &server.Q2A.config)
	if err != nil {
		log.Fatal(err)
	}

	rand.Seed(time.Now().Unix())

	log.Println("Loading private key:", server.Q2A.config.GetPrivateKey())
	privkey, err := LoadPrivateKey(server.Q2A.config.GetPrivateKey())
	if err != nil {
		log.Fatalf("Problems loading private key: %s\n", err.Error())
	}

	pubkey := privkey.Public().(*rsa.PublicKey)
	server.Q2A.privatekey = privkey
	server.Q2A.publickey = pubkey

	server.DB = DatabaseConnect()

	rules, err := FetchRules("rules.q2a")
	if err != nil {
		log.Println(err)
	} else {
		server.Q2A.rules = rules
	}

	log.Println("Loading clients from:", server.Q2A.config.GetClientFile())
	clients, err := LoadClients(server.Q2A.config.GetClientFile())
	if err != nil {
		log.Println(err)
	} else {
		server.Q2A.clients = clients
	}

	// Read users
	log.Println("Loading users from:", server.Q2A.config.GetUserFile())
	users, err := ReadUsersFromDisk(server.Q2A.config.GetUserFile())
	if err != nil {
		log.Println(err)
	} else {
		server.Q2A.Users = users
	}

	for _, c := range server.Q2A.clients {
		log.Printf("server: %-25s [%s:%d]", c.Name, c.IPAddress, c.Port)
	}
}

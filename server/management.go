package server

import (
	"fmt"
	"log"
	"net"

	cpb "github.com/packetflinger/q2admind/proto/cmd"
	"google.golang.org/protobuf/encoding/prototext"
)

func startManagement() {
	port := fmt.Sprintf("%s:%d", srv.config.ManagementAddress, srv.config.ManagementPort)
	listener, err := net.Listen("tcp", port) // v4 + v6
	if err != nil {
		log.Println(err)
		return
	}
	defer listener.Close()

	log.Printf("Listening for management clients on %s\n", port)

	for {
		c, err := listener.Accept()
		if err != nil {
			log.Println(err)
			return
		}
		go handleManagementClient(c)
	}
}

func handleManagementClient(c net.Conn) {
	defer c.Close()
	input := make([]byte, 500)
	_, err := c.Read(input)
	if err != nil {
		log.Println("Management client read error:", err)
		return
	}
	var req cpb.HealthRequest
	err = prototext.Unmarshal(input, &req)
	if err != nil {
		log.Println(err)
		return
	}
}

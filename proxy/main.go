// cloudadmin-proxy is meant to protect the backend cloud-admin server
// by fronting all the TCP connections from Q2 servers.
//
// Ideally, this proxy can be run on a bunch of cheap VPS around the world
// and all connecting back to a central backend server. This way nobody
// knows the real IP of the backend so it can be safe from DDOS and other
// attacks.
package main

import (
	"flag"
	"io"
	"log"
	"net"
)

var (
	listen = flag.String("listen", "[::]:9988", "Listen on this port for incoming connections")
	target = flag.String("target", "dev.frag.gr:9988", "The backend server address")
)

func main() {
	flag.Parse()
	log.Println("Proxying", *listen, "->", *target)

	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		panic(err)
	}
	for {
		conn, err := listener.Accept()
		log.Println("New connection", conn.RemoteAddr())
		if err != nil {
			log.Println("error accepting connection", err)
			continue
		}
		go func() {
			defer conn.Close()
			conn2, err := net.Dial("tcp", *target)
			if err != nil {
				log.Println("error dialing remote addr", err)
				return
			}
			defer conn2.Close()
			closer := make(chan struct{}, 2)
			go copy(closer, conn2, conn)
			go copy(closer, conn, conn2)
			<-closer
			log.Println("Connection complete", conn.RemoteAddr())
		}()
	}
}

func copy(closer chan struct{}, dst io.Writer, src io.Reader) {
	_, _ = io.Copy(dst, src)
	closer <- struct{}{} // connection is closed, send signal to stop proxy
}

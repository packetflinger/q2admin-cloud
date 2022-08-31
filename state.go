package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
)

//
// Write and struct to disk for later use
//
func (srv *Server) SaveState() {
	fname := fmt.Sprintf("./states/%s.gob", srv.name)
	file, err := os.Create(fname)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	enc := gob.NewEncoder(file)
	enc.Encode(srv)
}

//
// Read a saved struct back into memory
//
func (srv *Server) LoadState() {
	fname := fmt.Sprintf("./states/%s.gob", srv.name)
	file, err := os.Open(fname)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	dec := gob.NewDecoder(file)
	err = dec.Decode(srv)
	log.Println(err)
}
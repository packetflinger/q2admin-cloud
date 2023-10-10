package client

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
)

// Write and struct to disk for later use
func (cl *Client) SaveState() {
	fname := fmt.Sprintf("./states/%s.gob", cl.Name)
	file, err := os.Create(fname)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	enc := gob.NewEncoder(file)
	enc.Encode(cl)
}

// Read a saved struct back into memory
func (cl *Client) LoadState() {
	fname := fmt.Sprintf("./states/%s.gob", cl.Name)
	file, err := os.Open(fname)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	dec := gob.NewDecoder(file)
	err = dec.Decode(cl)
	log.Println(err)
}

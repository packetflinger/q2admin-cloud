package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Grab some of the incoming message buffer
func ReadData(msg *MessageBuffer, length int) []byte {
	start := msg.index
	msg.index += length
	return msg.buffer[start:msg.index]
}

/**
 * Keep building a string until we hit a null
 */
func ReadString(msg *MessageBuffer) string {
	var buffer bytes.Buffer

	// find the next null (terminates the string)
	for i := 0; msg.buffer[msg.index] != 0; i++ {
		// we hit the end without finding a null
		if msg.index == len(msg.buffer) {
			break
		}

		buffer.WriteString(string(msg.buffer[msg.index]))
		msg.index++
	}

	msg.index++
	return buffer.String()
}

/**
 * Read 4 bytes as a Long
 */
func ReadLong(msg *MessageBuffer) int32 {
	var tmp struct {
		Value int32
	}

	r := bytes.NewReader(msg.buffer[msg.index:])
	if err := binary.Read(r, binary.LittleEndian, &tmp); err != nil {
		fmt.Println("binary.Read failed:", err)
	}

	msg.index += 4
	return tmp.Value
}

/**
 * Read two bytes as a Short
 */
func ReadShort(msg *MessageBuffer) int16 {
	var tmp struct {
		Value int16
	}

	r := bytes.NewReader(msg.buffer[msg.index:])
	if err := binary.Read(r, binary.LittleEndian, &tmp); err != nil {
		fmt.Println("binary.Read failed:", err)
	}

	msg.index += 2
	return tmp.Value
}

// for consistency
func ReadByte(msg *MessageBuffer) byte {
	val := byte(msg.buffer[msg.index])
	msg.index++
	return val
}

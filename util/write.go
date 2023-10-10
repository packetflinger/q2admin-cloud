package util

import (
	//"bytes"
	"encoding/binary"
)

/**
 * Append to the end of our msg buffer
 */
func WriteData(data []byte, msg *MessageBuffer) {
	msg.buffer = append(msg.buffer, data...)
	msg.length += len(data)
}

/**
 * Keep building a string until we hit a null
 */
func WriteString(data string, msg *MessageBuffer) {
	b := []byte(data)
	b = append(b, []byte{0}...) // null terminated
	msg.buffer = append(msg.buffer, b...)
	msg.length += len(b)
}

/**
 * Write 4 byte int into to LE buffer
 */
func WriteLong(data int, msg *MessageBuffer) {
	temp := make([]byte, 4)
	binary.LittleEndian.PutUint32(temp, uint32(data)&0xffffffff)
	msg.buffer = append(msg.buffer, temp...)
	msg.length += 4
}

/**
 * Write 2 byte int into LE buffer
 */
func WriteShort(data int, msg *MessageBuffer) {
	temp := make([]byte, 2)
	binary.LittleEndian.PutUint16(temp, uint16(data)&0xffff)
	msg.buffer = append(msg.buffer, temp...)
	msg.length += 2
}

// for consistency
func WriteByte(data byte, msg *MessageBuffer) {
	temp := make([]byte, 1)
	temp[0] = data & 0xff
	//msg.buffer = append(msg.buffer, data & 0xff)
	msg.buffer = append(msg.buffer, temp...)
	msg.length++
}

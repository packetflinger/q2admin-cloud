package main

import (
    "bytes"
    "encoding/binary"
)

/**
 * Append to the end of our msg buffer
 */
func WriteData(msg *MessageBuffer, data []byte) {
    msg.buffer = append(msg.buffer, data...)
}

/**
 * Keep building a string until we hit a null
 */
func WriteString(msg *MessageBuffer, data string) {
    b := []byte(data)
    b = append(b, []byte{0}...) // null terminated
    msg.buffer = append(msg.buffer, b...)
}

/**
 * Write 4 byte int into to LE buffer
 */
func WriteLong(msg *MessageBuffer, data int32) {
    temp := new(bytes.Buffer)
    _ = binary.Write(temp, binary.LittleEndian, data)
    msg.buffer = append(msg.buffer, temp.Bytes()...)
}

/**
 * Write 2 byte int into LE buffer
 */
func WriteShort(msg *MessageBuffer, data int16) {
    temp := new(bytes.Buffer)
    _ = binary.Write(temp, binary.LittleEndian, data)
    msg.buffer = append(msg.buffer, temp.Bytes()...)
}

// for consistency
func WriteByte(msg *MessageBuffer, data byte) {
    msg.buffer = append(msg.buffer, data)
}

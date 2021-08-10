package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
    "encoding/binary"
    "bytes"
)

type MessageBuffer struct {
	buffer []byte
	index  int32
	length int32 // maybe not needed
}

type Player struct {
	clientid     int8
	name         string
	userinfo     string
	frags        int16
	deaths       int16
	suicides     int16
	teleports    int16
	lastteleport int32
}

// this is a Quake 2 Gameserver, and also a client to us
type Server struct {
	id         int32 // this is the database index
	key        int32
	index      int32
	version    int32 // what version are we running
	ipaddress  string
	port       int16 // default 27910
	connected  bool
	currentmap string
	enabled    bool
	connection *net.Conn
	players    []Player
	message    MessageBuffer
}

func handleConnection(c net.Conn) {
	fmt.Printf("Serving %s\n", c.RemoteAddr().String())
	for {
		netData, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			fmt.Println(err)
			return
		}

		temp := strings.TrimSpace(string(netData))
		if temp == "STOP" {
			break
		}

		result := strconv.Itoa(rand.Intn(100)) + "\n"
		c.Write([]byte(string(result)))
	}
	c.Close()
}

func main() {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide a port number!")
		return
	}

	port := ":" + arguments[1]
	listener, err := net.Listen("tcp", port) // v4 + v6
	if err != nil {
		fmt.Println(err)
		return
	}
	defer listener.Close()

	rand.Seed(time.Now().Unix())

	for {
		c, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		go handleConnection(c)
	}
}

func init() {
    data := []byte{0x60,0x05,0x00,0x00,0x0C,0x22,0x00,0x00,0x00,0xCD,0x62,0xF0,0x0A,0x01,0x6F,0x70,0x65,0x6E,0x74,0x64,0x6D,0x00}
    //var msg MessageBuffer
    //msg.buffer = data
    //msg := new(MessageBuffer{buffer:data, index:0, length:len(data)})
    fmt.Printf("Long value: %d\n", ReadLong(data[4:]))
    fmt.Printf("Short value: %d\n", ReadShort(data[9:]))
    fmt.Printf("Byte value: %d\n", ReadByte(data[11:]))
    fmt.Printf("String value: %s\n", ReadString(data[14:]))

    os.Exit(1)
}

func ReadString(d []byte) string {
    var buffer bytes.Buffer

    // find the next null (terminates the string)
    for i:=0; d[i]!=0; i++ {
        buffer.WriteString(string(d[i]))
    }

    return buffer.String()
}

func ReadLong(d []byte) int32 {
    var tmp struct {
        Value int32
    }

    r := bytes.NewReader(d)
    if err := binary.Read(r, binary.LittleEndian, &tmp); err != nil {
        fmt.Println("binary.Read failed:", err)
    }

    return tmp.Value
}

func ReadShort(d []byte) int16 {
    var tmp struct {
        Value int16
    }

    r := bytes.NewReader(d)
    if err := binary.Read(r, binary.LittleEndian, &tmp); err != nil {
        fmt.Println("binary.Read failed:", err)
    }

    return tmp.Value
}

// for consistency
func ReadByte(d []byte) byte {
    return byte(d[0])
}

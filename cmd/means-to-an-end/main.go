package main

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
)

type client struct {
	messages map[int32]int32
}

func (c *client) insertMessage(timestamp, price int32) {
	if _, ok := c.messages[timestamp]; !ok {
		c.messages[timestamp] = price
		return
	}
	// behavior undefined
}

func (c *client) queryMean(minTime, maxTime int32) int {
	var prices []int32
	for t := range c.messages {
		if minTime <= t && t <= maxTime {
			prices = append(prices, t)
		}
	}

	var total int32
	for _, num := range prices {
		total += num
	}

	return int(total) / len(prices)
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	c := client{
		messages: make(map[int32]int32),
	}

	for {
		buf := make([]byte, 9)
		_, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			log.Fatalf("failed to read from connection: %v", err)
		}

		fint32 := int32(binary.BigEndian.Uint32(buf[1:4]))
		sint32 := int32(binary.BigEndian.Uint32(buf[5:8]))

		switch byte(buf[0]) {
		case 'I':
			c.insertMessage(fint32, sint32)
		case 'Q':
			c.queryMean(fint32, sint32)
		default:
			// undefined behavior
		}
	}
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("failed to accept connection: %v", err)
		}

		go handleConnection(conn)
	}
}

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

type client struct {
	messages map[int32]int32
}

func (c *client) insertMessage(timestamp, price int32) {
	if _, ok := c.messages[timestamp]; !ok {
		c.messages[timestamp] = price
	}
}

func (c *client) queryMeanPrice(minTime, maxTime int32) int32 {
	var prices []int32
	for t := range c.messages {
		if minTime <= t && t <= maxTime {
			prices = append(prices, c.messages[t])
		}
	}

	if len(prices) == 0 {
		return 0
	}

	var total int
	for _, num := range prices {
		total += int(num)
	}

	return int32(total / len(prices))
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	c := client{
		messages: make(map[int32]int32),
	}

	for {
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 9)
		_, err := io.ReadFull(conn, buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			if errors.Is(err, os.ErrDeadlineExceeded) {
				return
			}
			fmt.Printf("failed to read from connection: %v", err)
			return
		}

		fint32 := int32(binary.BigEndian.Uint32(buf[1:5]))
		sint32 := int32(binary.BigEndian.Uint32(buf[5:]))

		switch byte(buf[0]) {
		case 'I':
			c.insertMessage(fint32, sint32)
		case 'Q':
			mean := c.queryMeanPrice(fint32, sint32)

			buf := make([]byte, 4)
			binary.BigEndian.PutUint32(buf, uint32(mean))
			if _, err := conn.Write(buf); err != nil {
				log.Fatalf("failed to write to connection: %v", err)
			}
		default:
			fmt.Printf("invalid type char, got %c, %v\n", buf[0], buf)
		}
	}
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()
	log.Println("listening on port 8080")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("failed to accept connection: %v", err)
		}

		go handleConnection(conn)
	}
}

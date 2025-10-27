package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
)

const port = ":8080"

type request struct {
	key       string
	value     string
	isWrite   bool
	responses chan string
}

func dbManager(requests <-chan request) {
	db := make(map[string]string)
	db["version"] = "some stuff about the server"

	for req := range requests {
		if req.isWrite == true {
			if _, ok := db[req.key]; ok {
				fmt.Printf("overwriting %s with %s", req.key, req.value)
			}
			db[req.key] = req.value
		} else {
			res := db[req.key]
			req.responses <- res
		}
	}
}

func main() {
	addr, err := net.ResolveUDPAddr("udp", port)
	if err != nil {
		log.Fatalf("failed to resolve UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("failed to listen on UDP: %v", err)
	}
	defer conn.Close()

	fmt.Printf("listening on port %s\n", port)

	requests := make(chan request)
	go dbManager(requests)

	buf := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		// Issues related to UDP packets being dropped, delayed, or reordered are considered to be the client's problem.
		// The server should act as if it assumes that UDP works reliably.
		if err != nil {
			continue
		}

		// All requests and responses must be shorter than 1000 bytes.
		if n > 1000 {
			continue
		}

		data := make([]byte, n)
		copy(data, buf)
		data = bytes.TrimSpace(data)

		var req request
		index := bytes.Index(data, []byte{'='})
		if index != -1 {
			req.isWrite = true
			req.key = string(data[:index])
			req.value = string(data[index+1:])
			if req.key != "version" {
				requests <- req
			}
			continue
		}

		req.isWrite = false
		req.key = string(data)
		req.responses = make(chan string)
		requests <- req

		go func(ca *net.UDPAddr) {
			value := <-req.responses
			response := fmt.Sprintf("%s=%s", req.key, value)
			if _, err = conn.WriteToUDP([]byte(response), clientAddr); err != nil {
				log.Printf("DEBUG: WriteToUDP error: %v", err)
			}
		}(clientAddr)
	}
}

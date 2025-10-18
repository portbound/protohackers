package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"sync"
)

const port = ":8080"

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

	db := make(map[string]string)
	db["version"] = "some stuff about the server"
	mu := sync.Mutex{}

	for {
		buf := make([]byte, 1024)
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		// Create a copy of the data to be used in the goroutine
		data := make([]byte, n)
		copy(data, buf[:n])

		go func() {
			idx := bytes.Index(data, []byte{'='})
			if idx != -1 {
				key := string(data[:idx])
				if key == "version" {
					return
				}
				value := string(data[idx+1:])
				mu.Lock()
				db[key] = value
				mu.Unlock()
				return
			}

			key := string(data)
			mu.Lock()
			value := db[key]
			mu.Unlock()

			_, err := conn.WriteToUDP([]byte(value), clientAddr)
			if err != nil {
				// do nothing
			}
		}()
	}
}

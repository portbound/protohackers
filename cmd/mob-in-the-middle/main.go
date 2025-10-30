package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
)

const port = ":8080"
const bcPort = ":16963"
const tonysBGAddr = "7YWHMfk9JZe0LM0g1ZauHuiSxhI"

func sanitize(src, dst net.Conn) {
	scanner := bufio.NewScanner(src)
	for scanner.Scan() {
		var sanitizedMsg strings.Builder
		words := strings.SplitSeq(scanner.Text(), " ")
		for word := range words {
			if !strings.HasPrefix(word, "7") && (len(word) >= 26 && len(word) <= 35) {
				sanitizedMsg.WriteString(tonysBGAddr)
				continue
			}
			sanitizedMsg.WriteString(word)
		}
		if _, err := fmt.Fprintln(dst, sanitizedMsg); err != nil {
			log.Printf("write failed: %v", err)
			break
		}
	}
	if err := scanner.Err(); err != nil { // if EOF, scanner returns nil
		log.Printf("read failed: %v", err)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	upstream, err := net.Dial("tcp", bcPort)
	if err != nil {
		log.Printf("failed to connect to upstream: %v", err)
		return
	}
	defer upstream.Close()

	go sanitize(conn, upstream)
	go sanitize(upstream, conn)
}

func main() {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", port, err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("failed to accept connection to proxy server: %v", err)
		}

		go handleConnection(conn)
	}
}

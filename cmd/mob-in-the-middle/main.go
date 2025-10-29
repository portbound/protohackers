package main

import (
	"log"
	"net"
	"strings"
)

const port = ":8080"
const bcPort = ":16963"
const tonysBGAddr = "7YWHMfk9JZe0LM0g1ZauHuiSxhI"

func sanitize(msg string) string {
	var sanitizedMsg strings.Builder
	words := strings.SplitSeq(msg, " ")
	for word := range words {
		if !strings.HasPrefix(word, "7") && (len(word) >= 26 && len(word) <= 35) {
			sanitizedMsg.WriteString(tonysBGAddr)
			continue
		}

		sanitizedMsg.WriteString(word)
	}
	return sanitizedMsg.String()
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	bcListener, err := net.Listen("tcp", bcPort)
	if err != nil {
	}
	defer bcListener.Close()

	for {
		bcConn, err := bcListener.Accept()

	}

	// establish connection to real budget chat
	// when receiving message from client, run through sanitizer before passing to upstream
	// when receiving message from upstream, run throuhg sanitizer before passing to client
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
			log.Fatalf("failed to accept connection: %v", err)
		}

		go handleConnection(conn)
	}
}

package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
)

const port = ":8080"
const budgetChat = "chat.protohackers.com:16963"
const tonysBGAddr = "7YWHMfk9JZe0LM0g1ZauHuiSxhI"

func sanitize(src, dst net.Conn) {
	scanner := bufio.NewScanner(src)
	for scanner.Scan() {
		// if dst == nil {
		// 	break
		// }
		var strs []string
		words := strings.SplitSeq(scanner.Text(), " ")
		for word := range words {
			if strings.HasPrefix(word, "7") && (len(word) >= 26 && len(word) <= 35) {
				strs = append(strs, tonysBGAddr)
				continue
			}
			strs = append(strs, word)
		}

		msg := strings.Join(strs, " ")
		fmt.Println(msg)
		if _, err := dst.Write([]byte(msg + "\n")); err != nil {
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

	upstream, err := net.Dial("tcp", budgetChat)
	if err != nil {
		log.Printf("failed to connect to upstream: %v", err)
		return
	}
	defer upstream.Close()

	// need to handle these better. If someone leaves the room, the upstream disconnects, but the connection here never closes because we're still waiting
	var wg sync.WaitGroup
	wg.Go(func() {
		sanitize(conn, upstream)
	})
	wg.Go(func() {
		sanitize(upstream, conn)
	})
	wg.Wait()
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

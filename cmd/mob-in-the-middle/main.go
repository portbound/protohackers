package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
)

const port = ":8080"
const budgetChat = "chat.protohackers.com:16963"
const tonysBGAddr = "7YWHMfk9JZe0LM0g1ZauHuiSxhI"

func sanitizer(src, dst net.Conn) {
	r := bufio.NewReader(src)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Printf("read failed: %v", err)
			return
		}
		var strs []string
		words := strings.FieldsSeq(line)
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
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	upstream, err := net.Dial("tcp", budgetChat)
	if err != nil {
		log.Printf("failed to connect to upstream: %v", err)
		return
	}
	defer upstream.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		sanitizer(conn, upstream)
		upstream.Close()
	})
	wg.Go(func() {
		sanitizer(upstream, conn)
		conn.Close()
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

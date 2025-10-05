package main

import (
	"bufio"
	"encoding/json"
	"log"
	"math"
	"net"
)

type Request struct {
	Method string   `json:"method"`
	Number *float64 `json:"number"`
}

type Response struct {
	Method string `json:"method"`
	Prime  bool   `json:"prime"`
}

type MalformedResponse struct{}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		line := scanner.Bytes()
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			encoder.Encode(&MalformedResponse{})
			return
		}

		if req.Method != "isPrime" || req.Number == nil {
			encoder.Encode(&MalformedResponse{})
			return
		}

		if *req.Number != math.Trunc(*req.Number) {
			encoder.Encode(&Response{
				Method: "isPrime",
				Prime:  false,
			})
		} else {
			encoder.Encode(&Response{
				Method: "isPrime",
				Prime:  checkIsPrime(int64(*req.Number)),
			})
		}
	}
}

func checkIsPrime(n int64) bool {
	if n <= 1 {
		return false
	}
	if n == 2 {
		return true
	}
	if n%2 == 0 {
		return false
	}

	limit := int64(math.Sqrt(float64(n)))
	for i := int64(3); i <= limit; i += 2 {
		if n%i == 0 {
			return false
		}
	}
	return true
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to bind to port 8080: %v", err)
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

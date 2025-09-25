package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net"
)

type Request struct {
	Method string      `json:"method"`
	Number json.Number `json:"number"`
}

type Response struct {
	Method string `json:"method"`
	Prime  bool   `json:"prime"`
}

type MalformedResponse struct {
	Error string `json:"error"`
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {

		var req Request
		if err := decoder.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			encoder.Encode(&MalformedResponse{Error: err.Error()})
			return
		}

		if req.Method != "isPrime" || req.Number == "" { // checking req.Number may be redundant here? I dont think we can decode an empty string anyway
			encoder.Encode(&MalformedResponse{Error: "nope"})
			return
		}

		_, err := req.Number.Float64()
		if err != nil {
			encoder.Encode(&MalformedResponse{Error: err.Error()})
			return
		}

		var isPrime bool
		if n, err := req.Number.Int64(); err != nil {
			isPrime = false
		} else {
			isPrime = checkIsPrime(int(n))
		}

		_ = encoder.Encode(&Response{
			Method: "isPrime",
			Prime:  isPrime,
		})
	}
}

func checkIsPrime(n int) bool {
	if n <= 1 {
		return false
	}
	if n == 2 {
		return true
	}
	if n%2 == 0 {
		return false
	}

	limit := int(math.Sqrt(float64(n)))
	for i := 3; i <= limit; i += 2 {
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

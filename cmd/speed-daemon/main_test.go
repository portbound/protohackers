package main

import (
	"net"
	"reflect"
	"testing"
	"time"
)

func TestServer_HandleEvent(t *testing.T) {
	tests := []struct {
		name     string
		msg      Message
		setup    func(t *testing.T, e *Event, s *Server, c net.Conn)
		validate func(t *testing.T, e *Event, s *Server, c net.Conn)
	}{
		{
			name:  "Camera Msg Received: Success",
			msg:   &IAmCamera{Road: 123, Mile: 8, Limit: 60},
			setup: func(t *testing.T, e *Event, s *Server, c net.Conn) {},
			validate: func(t *testing.T, e *Event, s *Server, c net.Conn) {
				if len(s.Cameras) != 1 {
					t.Fatalf("expected 1 camera, got %d", len(s.Cameras))
				}
				camera, ok := s.Cameras[e.Conn]
				if !ok {
					t.Fatalf("failed to add client as camera")
				}

				if !reflect.DeepEqual(e.Msg, camera) {
					t.Fatalf("want: %+v, got: %+v", e.Msg, camera)
				}
			},
		},
		{
			name: "Dispatcher Msg Recieved: Success",
			msg: &IAmDispatcher{
				Numroads: 1,
				Roads:    []uint16{123},
			},
			setup: func(t *testing.T, e *Event, s *Server, c net.Conn) {},
			validate: func(t *testing.T, e *Event, s *Server, c net.Conn) {
				if len(s.Dispatchers) != 1 {
					t.Fatalf("expected 1 dispatcher, got %d", len(s.Dispatchers))
				}
				dispatcher, ok := s.Dispatchers[e.Conn]
				if !ok {
					t.Fatalf("failed to add client as dispatcher")
				}
				if !reflect.DeepEqual(e.Msg, dispatcher) {
					t.Fatalf("want: %+v, got: %+v", e.Msg, dispatcher)
				}
			},
		},
		{
			name: "WantHeartBeat Msg Received: Success",
			msg: &WantHeartbeat{
				Interval: 2,
			},
			setup: func(t *testing.T, e *Event, s *Server, c net.Conn) {},
			validate: func(t *testing.T, e *Event, s *Server, c net.Conn) {
				type readResult struct {
					data []byte
					err  error
				}
				ch := make(chan readResult)
				go func() {
					for {
						hb := make([]byte, 1)
						_, err := c.Read(hb)
						ch <- readResult{data: hb, err: err}
						if err != nil {
							return
						}
					}
				}()
				for range 4 {
					select {
					case result := <-ch:
						if result.err != nil {
							t.Fatalf("read failed: %v", result.err)
						}
					case <-time.After(2 * time.Second):
						t.Fatalf("timeout waiting for a heartbeat")
					}
				}
			},
		},
		{
			name: "Plate Msg Received: Success",
			msg: &Plate{
				Plate: Str{
					Len:  4,
					Body: []byte{'U', 'N', 'I', 'X'},
				},
				Timestamp: 0,
			},
			setup: func(t *testing.T, e *Event, s *Server, c net.Conn) {
				s.Cameras = make(map[net.Conn]*IAmCamera)
				camera := &IAmCamera{Road: 123, Mile: 8, Limit: 60}
				s.Cameras[e.Conn] = camera
			},
			validate: func(t *testing.T, e *Event, s *Server, c net.Conn) {
				camera := s.Cameras[e.Conn]
				road := s.Roads[camera.Road]

				if len(road) != 1 {
					t.Fatalf("expected 1 plate, got %d", len(road))
				}

				if _, ok := road["UNIX"]; !ok {
					t.Fatalf("plate UNIX was not added to map")
				}
			},
		},
		{
			name: "Plate Msg Received: Send a Ticket",
			msg: &Plate{
				Plate: Str{
					Len:  4,
					Body: []byte{'U', 'N', 'I', 'X'},
				},
				Timestamp: 45,
			},
			setup: func(t *testing.T, e *Event, s *Server, c net.Conn) {
				s.Cameras = make(map[net.Conn]*IAmCamera)
				camera := &IAmCamera{Road: 123, Mile: 9, Limit: 60}
				s.Cameras[e.Conn] = camera

				s.Roads[camera.Road] = make(map[string][]*Sighting)
				road := s.Roads[camera.Road]
				road["UNIX"] = append(road["UNIX"], &Sighting{
					Mile:      8,
					Timestamp: 0,
				})

				s.Dispatchers = make(map[net.Conn]*IAmDispatcher)
				dispatcher := &IAmDispatcher{
					Numroads: 1,
					Roads:    []uint16{123},
				}
				foo, conn := net.Pipe()
				defer foo.Close()
				defer conn.Close()
				s.Dispatchers[conn] = dispatcher
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer()
			c1, c2 := net.Pipe()
			defer c1.Close()
			defer c2.Close()

			e := &Event{
				Conn:   c2,
				Msg:    tt.msg,
				Signal: make(chan struct{}),
			}

			tt.setup(t, e, s, c1)
			s.HandleEvent(e)
			tt.validate(t, e, s, c1)
		})
	}
}

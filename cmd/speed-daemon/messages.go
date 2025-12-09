package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type typ uint8

const (
	MsgError         typ = 0x10
	MsgPlate         typ = 0x20
	MsgTicket        typ = 0x21
	MsgWantHeartBeat typ = 0x40
	MsgHeartBeat     typ = 0x41
	MsgIAmCamera     typ = 0x80
	MsgIAmDispatcher typ = 0x81
)

// Message Protocol Spec
// each message declares its type in a single uint8

type message interface {
	encode(net.Conn) error
	decode(net.Conn) error
}

type str struct {
	len  uint8
	body []byte
}

func (s *str) encode(conn net.Conn) error { return nil }
func (s *str) decode(conn net.Conn) error {
	err := binary.Read(conn, binary.BigEndian, &s.len)
	if err != nil {
		return fmt.Errorf("read str len err: %w", err)
	}

	s.body = make([]byte, s.len)
	_, err = io.ReadFull(conn, s.body)
	if err != nil {
		return fmt.Errorf("read str body err: %w", err)
	}

	return nil
}

type Error struct {
	msg str
}

func (e *Error) encode(conn net.Conn) error {
	err := e.msg.encode(conn)
	if err != nil {
		return fmt.Errorf("failed to encode str: %w", err)
	}
	err = binary.Write(conn, binary.BigEndian, &e)
	if err != nil {
		return fmt.Errorf("failed to write Error: %w", err)
	}
	return nil
}
func (e *Error) decode(conn net.Conn) error { return nil }

type Plate struct {
	plate     str
	timestamp uint32
}

func (p *Plate) encode(conn net.Conn) error { return nil }
func (p *Plate) decode(conn net.Conn) error {
	err := p.plate.decode(conn)
	if err != nil {
		return fmt.Errorf("failed to decode str: %w", err)
	}

	err = binary.Read(conn, binary.BigEndian, &p.timestamp)
	if err != nil {
		return fmt.Errorf("failed to read timestamp: %w", err)
	}

	return nil
}

type Ticket struct {
	plate      str
	road       uint16
	mile1      uint16
	timestamp1 uint32
	mile2      uint16
	timestamp2 uint32
	speed      uint16 // 100x mph
}

func (t *Ticket) encode(conn net.Conn) error {
	if err := binary.Write(conn, binary.BigEndian, t); err != nil {
		return err
	}
	return nil
}
func (t *Ticket) decode(conn net.Conn) error { return nil }

type WantHeartbeat struct {
	Interval uint32
}

func (w *WantHeartbeat) encode(conn net.Conn) error { return nil }
func (w *WantHeartbeat) decode(conn net.Conn) error {
	if err := binary.Read(conn, binary.BigEndian, w); err != nil {
		return err
	}

	return nil
}

type Heartbeat struct{}

func (h *Heartbeat) encode(conn net.Conn) error {
	if err := binary.Write(conn, binary.BigEndian, byte(MsgHeartBeat)); err != nil {
		return err
	}
	return nil
}
func (h *Heartbeat) decode(conn net.Conn) error { return nil }

type IAmCamera struct {
	road  uint16
	mile  uint16
	limit uint16
}

func (c *IAmCamera) encode(conn net.Conn) error { return nil }
func (c *IAmCamera) decode(conn net.Conn) error {
	if err := binary.Read(conn, binary.BigEndian, c); err != nil {
		return err
	}
	return nil
}

type IAmDispatcher struct {
	numroads uint8
	roads    []uint16
}

func (d *IAmDispatcher) encode(conn net.Conn) error { return nil }
func (d *IAmDispatcher) decode(conn net.Conn) error {
	if err := binary.Read(conn, binary.BigEndian, d); err != nil {
		return err
	}
	return nil
}

type ClientDisconnect struct{}

func (c *ClientDisconnect) encode(conn net.Conn) error { return nil }
func (c *ClientDisconnect) decode(conn net.Conn) error { return nil }

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"multi-player-game/game"
	"net"
	"strings"
	"sync"
)

const (
	NewPlayer               = "new-player"
	UpdatePlayerMovement    = "update-player-movement"
	DeletePlayer            = "delete-player"
	UpdatePlayerProjectiles = "update-player-projectiles"
)

type Server struct {
	port  string
	ln    net.Listener
	state game.State
	conns map[string]*Conn
	wg    sync.WaitGroup
	mu    sync.Mutex
}

type Conn struct {
	CurrPlayer *game.Player
	net.Conn
}

func NewConn(conn net.Conn) *Conn {
	return &Conn{
		Conn: conn,
	}
}

//func (c *Conn) HashAndSaltPassword(password string) error {
//	salt, err := bcrypt.GenerateSalt(10)
//	if err != nil {
//		return err
//	}
//
//	hashedPassword, err := bcrypt.HashPassword(password, salt)
//	if err != nil {
//		return err
//	}
//
//	c.HashedPass = hashedPassword
//	c.salt = salt
//	return nil
//}

func NewServer(port string) *Server {
	return &Server{
		port:  port,
		conns: make(map[string]*Conn),
		state: game.NewGameState(),
	}
}

type Message struct {
	payload []byte
	from    net.Addr
}

func (m Message) String() string {
	return fmt.Sprintf("from: %s\npayload: %s", m.from, m.payload)
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.port)
	if err != nil {
		return err
	}
	fmt.Println("server is listening on port", s.port)
	defer func() {
		err := ln.Close()
		if err != nil {
			fmt.Println("server closing error:", err)
		}
	}()

	s.ln = ln
	s.wg.Add(1)
	go s.acceptLoop()
	go s.writeLoop()
	s.wg.Wait()
	return nil
}

func (s *Server) readLoop(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			fmt.Println("connection closing error:", err)
		} else {
			s.mu.Lock()
			delete(s.conns, conn.RemoteAddr().String())
			s.mu.Unlock()
			fmt.Printf("connection closed (%s)\n", conn.RemoteAddr().String())
		}
	}(conn)
	payload := make([]byte, 2048)

	for {
		n, err := conn.Read(payload)
		if err != nil {
			if err == io.EOF {
				return
			}
			fmt.Println("read error:", err)
			continue
		}
		msg := Message{
			payload: payload[:n],
			from:    conn.RemoteAddr(),
		}

		fmt.Println("new message received:", msg)

		dst := &bytes.Buffer{}
		if err := json.Compact(dst, msg.payload); err != nil {
			fmt.Println("compact error", err)
		}
		msg.payload = dst.Bytes()
		fmt.Println("compacted:", string(msg.payload))
		err = s.handleMessage(msg, conn.RemoteAddr().String())
		if err != nil {
			write := fmt.Sprintln("error handling message:", err)
			fmt.Print(write)
			_, _ = conn.Write([]byte(write))
		}
	}
}

// precondition: parameter m's payload must be in the correct JSON format.
func (s *Server) handleMessage(m Message, addr string) error {
	// todo: change return messages to json format.
	payload := string(m.payload)
	data := make(map[string]any)
	err := json.Unmarshal([]byte(payload), &data)
	s.mu.Lock()
	conn := s.conns[addr]
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("error parsing json: %s", err)
	}

	if _, exists := data["request_type"]; !exists {
		return fmt.Errorf("json format is incorrect for json: %s\nexpected `request_type` attribute", strings.Trim(payload, "\n"))
	}

	requestType := data["request_type"]
	if requestType == NewPlayer {
		player, exists, _, err := s.state.HandleNewPlayer(data)
		if err != nil {
			fmt.Println("error creating new player:", err)
		}
		if exists {
			_, _ = conn.Write([]byte(fmt.Sprintf(
				`{"exists": true, "player_info":{"x": %f, "y": %f, "velocity_x": %f, "velocity_y": %f}}`,
				player.X, player.Y, player.VX, player.VY),
			))
		}
		conn.CurrPlayer = player
	} else if requestType == UpdatePlayerMovement {
		_, err := s.state.HandleUpdatePlayer(data, "movement")
		if err != nil {
			fmt.Println("error updating player:", err)
		}
	} else if requestType == DeletePlayer {
		_, err := s.state.HandleDeletePlayer(data)
		if err != nil {
			fmt.Println("error deleting player:", err)
		}
	} else if requestType == UpdatePlayerProjectiles {
		_, err := s.state.HandleUpdatePlayer(data, "projectiles")
		if err != nil {
			fmt.Println("error updating player:", err)
		}
	} else {
		msg := fmt.Sprintf("did not recieve valid `request_type` for json: %s", payload)
		_, _ = conn.Write([]byte(msg))
	}
	return nil
}

func (s *Server) writeLoop() {
	for {
		if len(s.state.Players) > 0 {
			msg, _ := json.Marshal(s.state.Players)
			err := s.broadcastMsg(string(msg) + "\n")
			if err != nil {
				fmt.Println("write loop err:", err)
			}
		}
	}
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.ln.Accept()
		if err != nil {
			fmt.Println("accept error:", err)
			continue
		}
		s.mu.Lock()
		s.conns[conn.RemoteAddr().String()] = NewConn(conn)
		fmt.Println(
			"new connection established:",
			conn.RemoteAddr().String(),
			"\ncurrent connections:",
			s.conns,
		)
		s.mu.Unlock()
		go s.readLoop(conn)
	}
}

func (s *Server) broadcastMsg(msg string) error {

	for addr := range s.conns {
		if s.conns[addr] == nil {
			continue
		}
		s.mu.Lock()
		_, err := s.conns[addr].Write([]byte(msg))
		s.mu.Unlock()
		if err != nil {
			fmt.Printf("write error writing to `%s`: %s\n", addr, err)
		}
	}
	return nil
}

func main() {
	server := NewServer(":3000")
	go func() {
		var input string
		for {
			_, err := fmt.Scanln(&input)
			if err != nil {
				fmt.Println("scan error:", err)
			}
			if input == "players" {
				marshal, _ := json.Marshal(server.state.Players)
				fmt.Println(string(marshal))
			}
		}
	}()
	err := server.Start()
	if err != nil {
		fmt.Println("start error:", err)
	}
}

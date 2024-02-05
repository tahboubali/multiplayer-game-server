package main

import (
	"encoding/json"
	"fmt"
	"io"
	"multi-player-game/game"
	"net"
	"strings"
	"sync"
)

const (
	newPlayer    = "new-player"
	updatePlayer = "update-player"
	deletePlayer = "delete-player"
)

type Server struct {
	port  string
	ln    net.Listener
	state game.GameState
	msgCh chan Message
	conns map[string]net.Conn
	wg    sync.WaitGroup
}

func NewServer(port string) *Server {
	return &Server{
		port:  port,
		msgCh: make(chan Message, 1),
		conns: make(map[string]net.Conn),
		state: game.NewGameState(),
	}
}

type Message struct {
	payload []byte
	from    net.Addr
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
		}
		fmt.Printf("connection closed (%s)\n", conn.RemoteAddr().String())
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

		err = s.handleMessage(msg, conn.RemoteAddr().String())
		if err != nil {
			write := fmt.Sprintln("error handling message:", err)
			fmt.Print(write)
			_, _ = conn.Write([]byte(write))
		}
		s.msgCh <- msg
	}
}

// Precondition: parameter m's payload must be in the correct JSON format.
func (s *Server) handleMessage(m Message, addr string) error {
	payload := string(m.payload)
	data := make(map[string]any)
	err := json.Unmarshal([]byte(payload), &data)
	conn := s.conns[addr]

	if err != nil {
		return fmt.Errorf("error parsing json: %s", err)

	}

	if _, exists := data["request_type"]; !exists {
		return fmt.Errorf("json format is incorrect for json: %s\nexpected request_type attribute", strings.Trim(payload, "\n"), data)
	}

	requestType := data["request_type"]
	if requestType == newPlayer {
		msg, err := s.state.HandleNewPlayer(data)
		if err != nil {
			fmt.Println("error creating new player:", err)
		}
		_ = s.broadcastMsg(string(msg))
	} else if requestType == updatePlayer {
		msg, err := s.state.HandleUpdatePlayer(data)
		if err != nil {
			fmt.Println("error updating player:", err)
		}
		_ = s.broadcastMsg(string(msg))
	} else if requestType == deletePlayer {
		msg, err := s.state.HandleDeletePlayer(data)
		if err != nil {
			fmt.Println("error deleting player:", err)
		}
		_ = s.broadcastMsg(string(msg))
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
		s.conns[conn.RemoteAddr().String()] = conn

		fmt.Println(
			"new connection established:",
			conn.RemoteAddr().String(),
			"\ncurrent connections:",
			s.conns,
		)

		go s.readLoop(conn)
	}
}

func (s *Server) broadcastMsg(msg string) error {
	for addr, conn := range s.conns {
		_, err := conn.Write([]byte(msg))
		if err != nil {
			return fmt.Errorf("write error writing to `%s`: %s", addr, err)
		}
	}
	return nil
}

func main() {
	server := NewServer(":3000")
	go func() {
		for msg := range server.msgCh {
			fmt.Printf("new message received from (%s): %s", msg.from, string(msg.payload))
		}
	}()
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

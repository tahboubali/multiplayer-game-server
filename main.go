package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"multi-player-game/consts"
	"multi-player-game/game"
	"multi-player-game/utils"
	"net"
	"strings"
	"sync"
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
	log.Println("server is listening on port", s.port)
	defer func() {
		err := ln.Close()
		if err != nil {
			log.Println("server closing error:", err)
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
			log.Println("connection closing error:", err)
		} else {
			s.mu.Lock()
			delete(s.conns, conn.RemoteAddr().String())
			s.mu.Unlock()
			log.Printf("connection closed (%s)\n", conn.RemoteAddr().String())
		}
	}(conn)
	payload := make([]byte, 2048)

	for {
		n, err := conn.Read(payload)
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Println("read error:", err)
			continue
		}
		msg := Message{
			payload: payload[:n],
			from:    conn.RemoteAddr(),
		}

		utils.DebugPrintln("new message received:", msg)

		dst := &bytes.Buffer{}
		if err := json.Compact(dst, msg.payload); err != nil {
			utils.DebugPrintln("compact error", err)
		}
		msg.payload = dst.Bytes()
		utils.DebugPrintln("compacted:", string(msg.payload))
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
	if requestType == consts.NewPlayer {
		player, exists, _, err := s.state.HandleNewPlayer(data)
		if err != nil {
			utils.DebugPrintln("error creating new player:", err)
		}
		if exists {
			_, _ = conn.Write([]byte(fmt.Sprintf(
				`{"exists": true, "player_info":{"x": %f, "y": %f, "velocity_x": %f, "velocity_y": %f}}`,
				player.X, player.Y, player.VX, player.VY),
			))
		}
		conn.CurrPlayer = player
	} else if requestType == consts.UpdatePlayerMovement {
		_, err := s.state.HandleUpdatePlayer(data, "movement")
		if err != nil {
			utils.DebugPrintln("error updating player:", err)
		}
	} else if requestType == consts.DeletePlayer {
		_, err := s.state.HandleDeletePlayer(data)
		if err != nil {
			utils.DebugPrintln("error deleting player:", err)
		}
	} else if requestType == consts.UpdatePlayerProjectiles {
		_, err := s.state.HandleUpdatePlayer(data, "projectiles")
		if err != nil {
			utils.DebugPrintln("error updating player:", err)
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
				utils.DebugPrintln("write loop err:", err)
			}
		}
	}
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.ln.Accept()
		if err != nil {
			utils.DebugPrintln("accept error:", err)
			continue
		}
		s.mu.Lock()
		s.conns[conn.RemoteAddr().String()] = NewConn(conn)
		utils.DebugPrintln(
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
			log.Printf("write error writing to `%s`: %s\n", addr, err)
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
				log.Println("scan error:", err)
			}
			if input == "players" {
				marshal, _ := json.Marshal(server.state.Players)
				log.Println(string(marshal))
			}
		}
	}()
	err := server.Start()
	if err != nil {
		log.Println("start error:", err)
	}
}

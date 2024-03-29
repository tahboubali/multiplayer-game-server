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
	"time"
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
	go s.handleCoinUpdates()
	s.wg.Wait()
	return nil
}

func (s *Server) handleCoinUpdates() {
	for {
		s.state.GenerateCoin()
		s.broadcastCoinUpdate()
		time.Sleep(5 * time.Second)
	}
}

func (s *Server) broadcastCoinUpdate() {
	data := make(map[string]any)
	data["request_type"] = consts.CoinUpdate
	data["coin_info"] = s.state.Coin
	marshal, err := json.Marshal(data)
	if err != nil {
		utils.DebugLog("marshal error:", err)
		return
	}
	if err := s.broadcastMsg(marshal); err != nil {
		utils.DebugLog("coin broadcast err:", err)
	}
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
	buf := make([]byte, 2048)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Println("read error:", err)
			continue
		}

		msg := Message{
			payload: buf[:n],
			from:    conn.RemoteAddr(),
		}
		utils.DebugLog("new message received:", msg)

		dst := &bytes.Buffer{}
		if err := json.Compact(dst, msg.payload); err != nil {
			utils.DebugLog("compact error", err)
		}
		msg.payload = dst.Bytes()
		utils.DebugLog("compacted:", string(msg.payload))
		err = s.handleMessage(msg, conn.RemoteAddr().String())

		if err != nil {
			write := fmt.Sprintln("error handling message:", err)
			fmt.Print(write)
			_, _ = conn.Write([]byte(write))
		}
	}
}

func (s *Server) handleCoinUpdates() {
	for {
		s.state.GenerateCoin()
		s.broadcastCoinUpdate()
		time.Sleep(5 * time.Second)
	}
}

func (s *Server) broadcastCoinUpdate() {
	data := make(map[string]any)
	data["request_type"] = consts.CoinUpdate
	data["coin_info"] = s.state.Coin
	marshal, err := json.Marshal(data)
	if err != nil {
		utils.DebugLog("marshal error:", err)
		return
	}
	if err := s.broadcastMsg(string(marshal)); err != nil {
		utils.DebugLog("coin broadcast err:", err)
	}
}

// precondition: parameter m's payload must be in the correct JSON format.
func (s *Server) handleMessage(m Message, addr string) error {
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
			utils.DebugLog("error creating new player:", err)
		}
		if exists {
			_, _ = conn.Write([]byte(fmt.Sprintf(
				`{"request_type": "%s", "exists": true, "player_info":{"x": %f, "y": %f, "velocity_x": %f, "velocity_y": %f}}`,
				consts.NewPlayer, player.X, player.Y, player.VX, player.VY),
			))
		}
		conn.CurrPlayer = player
		s.broadcastPlayers()
	} else if requestType == consts.UpdatePlayerMovement {
		_, err := s.state.HandleUpdatePlayer(data, "movement")
		if err != nil {
			utils.DebugLog("error updating player:", err)
		}
		s.broadcastPlayers()
	} else if requestType == consts.DeletePlayer {
		_, deleted, err := s.state.HandleDeletePlayer(data)
		if err != nil {
			utils.DebugLog("error deleting player:", err)
		} else {
			s.broadcastDelete(deleted)
		}
	} else {
		msg := fmt.Sprintf("did not recieve valid `request_type` for json: %s", payload)
		_, _ = conn.Write([]byte(msg))
		s.broadcastPlayers()
	}
	return nil
}

func (s *Server) broadcastPlayers() {
	if len(s.state.Players) > 0 {
		data := make(map[string]any)
		data["request_type"] = consts.UpdatePlayers
		data["players"] = s.state.Players
		msg, err := json.Marshal(data)
		if err != nil {
			log.Println("marshal error:", err)
			return
		}
		if err := s.broadcastMsg(msg); err != nil {
			log.Println("broadcast error:", err)
			return
		}
	}
}

func (s *Server) broadcastDelete(username string) {
	data := make(map[string]any)
	data["request_type"] = consts.DeletePlayer
	data["username"] = username
	marshal, err := json.Marshal(data)
	if err != nil {
		log.Println("error marshaling json:", err)
		return
	}
	if err := s.broadcastMsg(marshal); err != nil {
		utils.DebugLog("error broadcasting message:", err)
	}
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.ln.Accept()
		if err != nil {
			utils.DebugLog("accept error:", err)
			continue
		}
		s.mu.Lock()
		s.conns[conn.RemoteAddr().String()] = NewConn(conn)
		log.Println(
			"new connection established:",
			conn.RemoteAddr().String(),
			"\ncurrent connections:",
			s.conns,
		)
		s.mu.Unlock()
		go s.readLoop(conn)
		go s.handleCoinUpdates()
	}
}

func (s *Server) broadcastMsg(msg []byte) error {
	for addr := range s.conns {
		s.mu.Lock()
		if s.conns[addr] == nil {
			continue
		}
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

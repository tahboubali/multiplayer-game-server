package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
)

const (
	CoinRadius = 30
	Width      = 750
	Height     = 500
)

type State struct {
	playersLock sync.Mutex
	Players     map[string]*Player
	Coin        Coin
}

type gameObj struct {
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	VX float64 `json:"velocity_x"`
	VY float64 `json:"velocity_y"`
}

type Coin struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func NewGameState() State {
	return State{
		Players: make(map[string]*Player),
	}
}

func (g *State) GenerateCoin() {
	g.Coin.X = float64(rand.Intn(Width - CoinRadius + 1))
	g.Coin.Y = float64(rand.Intn(Height - CoinRadius + 1))
}

// HandleUpdatePlayer updateType refers to what aspect should be updated. for now, the two update types are movement and projectiles
func (g *State) HandleUpdatePlayer(data map[string]any, updateType string) ([]byte, error) {
	g.playersLock.Lock()
	defer g.playersLock.Unlock()
	player, err := extractPlayerInfo(data)
	if err != nil {
		return []byte(fmt.Sprintf("failed to update player: %s\n", err.Error())), err
	}
	username := player.Username
	if _, exists := g.Players[username]; !exists {
		return []byte(fmt.Sprintf("failed to update player: player with username %s does not exist\n", username)),
			fmt.Errorf("player with username `%s` does not exist", username)
	}
	if err != nil {
		return []byte(fmt.Sprintf("failed to update player projectiles: %s\n", err.Error())), err
	}
	if updateType == "movement" {
		*(g.Players[username]) = *player
	}
	marshal, _ := json.Marshal(*player)

	return []byte(fmt.Sprintf("{\"message\": \"update_player\", \"player\":%v}\n", marshal)), nil
}

func (g *State) HandleDeletePlayer(data map[string]any) ([]byte, string, error) {
	g.playersLock.Lock()
	defer g.playersLock.Unlock()
	username, ok := data["username"].(string)
	if !ok {
		return []byte("failed to delete player\n"), "", fmt.Errorf("invalid type for username")
	}
	if _, exists := g.Players[username]; !exists {
		return []byte("failed to delete player\n"), "", fmt.Errorf("player with username `%s` does not exist", username)
	}
	delete(g.Players, username)
	return []byte(fmt.Sprintf("{\"message\": \"delete_player\", \"username\":%v}\n", username)), username, nil
}

func (g *State) HandleNewPlayer(data map[string]any) (*Player, bool, []byte, error) {
	g.playersLock.Lock()
	defer g.playersLock.Unlock()
	player, err := extractPlayerInfo(data)
	if err != nil {
		return nil, false, []byte(fmt.Sprintf("failed to create player, %s\n", err.Error())), err
	}
	if _, exists := g.Players[player.Username]; exists {
		return g.Players[player.Username], true, []byte(fmt.Sprintf("player with username `%s` exists", player.Username)), nil
	}
	g.Players[player.Username] = player
	players, _ := json.Marshal(*player)
	coinInfo, _ := json.Marshal(g.Coin)
	return player, false, []byte(fmt.Sprintf("{\"message\": \"new_player\", \"player\":%s, \"coin_info\":%s}\n", string(players), string(coinInfo))), nil
}

func extractPlayerInfo(data map[string]any) (*Player, error) {
	if _, exists := data["player_info"]; !exists {
		marshal, _ := json.Marshal(data)
		return nil, fmt.Errorf("`player_info` attribute not found in message for json:\n%s", string(marshal))
	}

	playerInfo, err := getAttributeFromData[map[string]any](data, "player_info")
	if err != nil {
		return nil, err
	}
	username, err := getAttributeFromData[string](playerInfo.(map[string]any), "username")
	if err != nil {
		return nil, err
	}
	x, err := getAttributeFromData[float64](playerInfo.(map[string]any), "x")
	if err != nil {
		return nil, err
	}
	y, err := getAttributeFromData[float64](playerInfo.(map[string]any), "y")
	if err != nil {
		return nil, err
	}
	vX, err := getAttributeFromData[float64](playerInfo.(map[string]any), "velocity_x")
	if err != nil {
		return nil, err
	}
	vY, err := getAttributeFromData[float64](playerInfo.(map[string]any), "velocity_y")
	if err != nil {
		return nil, err
	}
	player := NewPlayer(username.(string), x.(float64), y.(float64))
	player.VX = vX.(float64)
	player.VY = vY.(float64)
	return player, nil
}

func getAttributeFromData[T any](data map[string]any, attrName string) (any, error) {
	if _, exists := data[attrName]; !exists {
		if attrName == "velocity_x" || attrName == "velocity_y" {
			return 0.0, nil
		}
		return nil, fmt.Errorf("attribute `%s` does not exist", attrName)
	}
	attr, ok := data[attrName].(T)
	if !ok {
		return nil, fmt.Errorf("invalid type for `%s`", attrName)
	}

	return attr, nil
}

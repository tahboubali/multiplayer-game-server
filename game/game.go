package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
)

const (
	COIN_RADIUS = 10
	WIDTH       = 750
	HEIGHT      = 500
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
	gameObj
}

func NewGameState() State {
	return State{
		Players: make(map[string]*Player),
	}
}

func (g *State) GenerateCoin() {
	g.Coin.X = float64(rand.Intn(WIDTH - COIN_RADIUS + 1))
	g.Coin.Y = float64(rand.Intn(HEIGHT - COIN_RADIUS + 1))
}

func (g *State) handleProj(username string, data map[string]any) error {
	if _, exists := data["projectiles"]; !exists {
		return fmt.Errorf("`projectiles` attribute not found")
	}
	projs, ok := data["projectiles"].(map[int64]any)
	if !ok {
		return fmt.Errorf("invalid type for attribute `projectiles`")
	}
	g.playersLock.Lock()
	player := g.Players[username]
	g.playersLock.Unlock()
	projectiles := player.Projectiles
	err := validateProjectiles(projs)
	if err != nil {
		return err
	}
	for id, tmp := range projs {
		projData := tmp.(map[string]float64)
		x := projData["x"]
		y := projData["y"]
		vX := projData["velocity_x"]
		vY := projData["velocity_y"]
		if projectiles[id] != (Projectile{}) {
			player.ShootProj(x, y, vX, vY)
		} else {
			proj := NewProjectile(username, x, y, vX, vY)
			projectiles[id] = proj
			proj.Id = id
			// FIXME might not work.
		}
	}
	return nil
}

func validateProjectiles(data map[int64]any) error {
	for id, proj := range data {
		err := validateData(id, proj, []string{"x", "y", "velocity_x", "velocity_y"})
		if err != nil {
			return err
		}
	}
	return nil
}

func validateData(id int64, data any, attributes []string) error {
	valid, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid type for projectile id %d", id)
	}

	for _, attr := range attributes {
		if _, exists := valid[attr]; exists {
			return fmt.Errorf("`%s` attribute not found in projectile for id %d", attr, id)
		}
	}
	return nil
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
	if updateType == "projectile" {
		err := g.handleProj(username, data)
		if err != nil {
			return []byte(fmt.Sprintf("failed to update player: %s\n", err.Error())), err
		}
		*(g.Players[username]) = *player
	} else if updateType == "movement" {
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
	marshal, _ := json.Marshal(*player)
	return player, false, []byte(fmt.Sprintf("{\"message\": \"new_player\", \"player\":%v}\n", string(marshal))), nil
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

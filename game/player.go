package game

import (
	"fmt"
	"multi-player-game/utils"
	"sync"
)

var (
	idLock     sync.Mutex
	currProjId int64
)

type Player struct {
	Projectiles []Projectile `json:"projectiles"`
	Username    string       `json:"username"`
	Score       int          `json:"score"`
	gameObj
}

func (p *Player) String() string {
	s, err := utils.PrettyStruct(p)
	if err != nil {
		fmt.Printf("error with String() for player `%s`\n", p.Username)
	}
	return s
}

func (p *Player) ShootProj(x, y, vX, vY float64) {
	proj := NewProjectile(p.Username, x, y, vX, vY)
	p.Projectiles = append(p.Projectiles, proj)
}

func NewPlayer(username string, x float64, y float64) *Player {
	return &Player{
		Username:    username,
		Projectiles: make([]Projectile, 0),
		gameObj: gameObj{
			X: x,
			Y: y,
		},
	}
}

type Projectile struct {
	Id      int64  `json:"id"`
	Shooter string `json:"shooter"`
	gameObj
}

func (p Projectile) String() string {
	s, _ := utils.PrettyStruct(p)
	return s
}

func NewProjectile(shooter string, x, y, vX, vY float64) Projectile {
	idLock.Lock()
	currProjId++
	idLock.Unlock()
	projectile := Projectile{
		Shooter: shooter,
		Id:      currProjId,
		gameObj: gameObj{
			X:  x,
			Y:  y,
			VX: vX,
			VY: vY,
		},
	}
	return projectile
}

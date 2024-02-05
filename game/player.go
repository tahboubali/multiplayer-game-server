package game

import "sync"

var (
	idLock     sync.Mutex
	currProjId int64
)

type gameObj struct {
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	VX float64 `json:"velocity_x"`
	VY float64 `json:"velocity_y"`
}

type Player struct {
	Projectiles []Projectile `json:"projectiles"`
	Username    string       `json:"username"`
	gameObj
}

func (p Player) ShootProj(x, y, vX, vY float64) {
	proj := NewProjectile(p.Username, x, y, vX, vY)
	proj.Id = currProjId
	p.Projectiles = append(p.Projectiles, proj)
}

func NewPlayer(username string, x float64, y float64) Player {
	return Player{
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

func NewProjectile(shooter string, x, y, vX, vY float64) Projectile {
	projectile := Projectile{
		Shooter: shooter,
		gameObj: gameObj{
			X:  x,
			Y:  y,
			VX: vX,
			VY: vY,
		},
	}
	idLock.Unlock()
	currProjId++
	idLock.Lock()
	return projectile
}

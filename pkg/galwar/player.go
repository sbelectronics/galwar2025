package galwar

import (
	"strings"

	"github.com/google/uuid"
	"github.com/sbelectronics/galwar/pkg/moderation"
	"golang.org/x/crypto/bcrypt"
)

type PlayerId string

type Player struct {
	Id        PlayerId
	Email     string
	GoogleSub string // Google OIDC subject; "" for accounts that predate web auth
	PassHash  string // bcrypt hash for telnet logins; "" = no telnet password set
	LastSeen  int64  // unix seconds of last session start
	ObjectBase
	InventoryBase
}

type PlayerList struct {
	Players []*Player
}

// NewPlayer creates a player record with the starting ship. Most callers
// want RegisterPlayer, which also enforces handle moderation and uniqueness.
func (u *UniverseType) NewPlayer(name string, email string) *Player {
	p := &Player{
		Id:    PlayerId(uuid.New().String()),
		Email: email,
		ObjectBase: ObjectBase{
			Name:   name,
			Sector: 1,
		},
		InventoryBase: InventoryBase{
			Money: u.ConfigInt("starting_credits", 35000),
		},
	}

	for _, tg := range TradeGoods {
		quantity := tg.Starting
		switch tg.Name {
		case HOLDS:
			quantity = u.ConfigInt("starting_holds", quantity)
		case FIGHTERS:
			quantity = u.ConfigInt("starting_fighters", quantity)
		case TURNS:
			quantity = u.ConfigInt("turns_per_day", quantity)
		}
		cm := Commodity{
			Name:     tg.Name,
			Quantity: quantity,
		}
		p.Inventory = append(p.Inventory, &cm)
	}

	u.Players.Players = append(u.Players.Players, p)
	u.MarkDirty()

	return p
}

// RegisterPlayer is the engine-side new-player command: the handle passes
// the moderation pipeline and must be unique in normalized form (so "Scott",
// "sc0tt", and "S c o t t" collide - the anti-impersonation rule).
func (u *UniverseType) RegisterPlayer(name string, email string, googleSub string) (*Player, error) {
	name = strings.TrimSpace(name)
	if err := moderation.CheckName(name); err != nil {
		return nil, NewGameError(ErrInvalidName, err.Error())
	}
	norm := moderation.Normalize(name)
	for _, p := range u.Players.Players {
		if moderation.Normalize(p.Name) == norm {
			return nil, NewGameError(ErrAlreadyExists, "That handle is already taken.")
		}
	}
	p := u.NewPlayer(name, email)
	p.GoogleSub = googleSub
	u.MarkDirty()
	return p, nil
}

// SetTelnetPassword sets the player's password for telnet logins.
func (u *UniverseType) SetTelnetPassword(p *Player, password string) error {
	if len(password) < 6 {
		return NewGameError(ErrInvalidName, "Passwords must be at least 6 characters.")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	p.PassHash = string(hash)
	u.MarkDirty()
	return nil
}

// CheckTelnetPassword verifies a telnet login attempt. Accounts without a
// password (web-only accounts) always fail.
func (p *Player) CheckTelnetPassword(password string) bool {
	if p.PassHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(p.PassHash), []byte(password)) == nil
}

// TouchLastSeen records a session start.
func (u *UniverseType) TouchLastSeen(p *Player, now int64) {
	p.LastSeen = now
	u.MarkDirty()
}

func (p *Player) GetNameExtra() string {
	return ""
}

func (p *Player) GetType() string {
	return TYPE_PLAYER
}

func (p *PlayerList) GetObjectsInSector(sector int) []ObjectInterface {
	var playersInSector []ObjectInterface
	for _, player := range p.Players {
		if player.Sector == sector {
			playersInSector = append(playersInSector, player)
		}
	}
	return playersInSector
}

func (p *PlayerList) GetByEmail(email string) *Player {
	for _, p := range p.Players {
		if p.Email == email {
			return p
		}
	}
	return nil
}

func (p *PlayerList) GetBySub(sub string) *Player {
	if sub == "" {
		return nil
	}
	for _, p := range p.Players {
		if p.GoogleSub == sub {
			return p
		}
	}
	return nil
}

// GetByNormalizedName finds a player by moderation-normalized handle, the
// comparison used for uniqueness and for telnet logins.
func (p *PlayerList) GetByNormalizedName(name string) *Player {
	norm := moderation.Normalize(name)
	if norm == "" {
		return nil
	}
	for _, p := range p.Players {
		if moderation.Normalize(p.Name) == norm {
			return p
		}
	}
	return nil
}

func (p *PlayerList) GetById(id PlayerId) *Player {
	for _, p := range p.Players {
		if p.Id == id {
			return p
		}
	}
	return nil
}

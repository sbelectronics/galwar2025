package galwar

import (
	"fmt"
)

type GameErrorCode int

const (
	ErrUnknown GameErrorCode = iota
	ErrNegativeQuantity
	ErrNotEnoughMoney
	ErrNotEnoughQuantity
	ErrNotOwner
	ErrInvalidType
	ErrFedRestricted
	ErrAlreadyExists
	ErrInvalidName
	ErrNotFound
	ErrNotEnoughHolds
	ErrNoTurns
	ErrDead
)

var GameErrorCodeNames = map[GameErrorCode]string{
	ErrUnknown:           "Unknown error",
	ErrNegativeQuantity:  "Negative quantity",
	ErrNotEnoughMoney:    "Not enough money",
	ErrNotEnoughQuantity: "Not enough quantity",
	ErrNotOwner:          "Not owner",
	ErrInvalidType:       "Invalid type",
	ErrFedRestricted:     "Federation restricted",
	ErrAlreadyExists:     "Already exists",
	ErrInvalidName:       "Invalid name",
	ErrNotFound:          "Not found",
	ErrNotEnoughHolds:    "Not enough holds",
	ErrNoTurns:           "No turns left",
	ErrDead:              "Ship destroyed",
}

type GameError struct {
	code    GameErrorCode
	message string
}

func (e *GameError) Error() string {
	return fmt.Sprintf("Error %s: %s", GameErrorCodeNames[e.code], e.message)
}

func (e *GameError) Message() string {
	return e.message
}

func NewGameError(code GameErrorCode, message string) *GameError {
	return &GameError{
		code:    code,
		message: message,
	}
}

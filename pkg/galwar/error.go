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
)

var GameErrorCodeNames = map[GameErrorCode]string{
	ErrUnknown:           "Unknown error",
	ErrNegativeQuantity:  "Negative quantity",
	ErrNotEnoughMoney:    "Not enough money",
	ErrNotEnoughQuantity: "Not enough quantity",
}

type GameError struct {
	code    GameErrorCode
	message string
}

func (e *GameError) Error() string {
	return fmt.Sprintf("Error %s: %s", GameErrorCodeNames[e.code], e.Message)
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

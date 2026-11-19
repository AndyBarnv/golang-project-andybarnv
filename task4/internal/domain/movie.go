package domain

import (
	"errors"
)

var (
	ErrBadRange = errors.New("invalid range")
	ErrNotFound = errors.New("movie not found")
)

type Movie struct {
	ID       int
	Title    string
	Year     int
	Director string
}

type MovieItem struct {
	ID    int
	Movie Movie
	Error error
}

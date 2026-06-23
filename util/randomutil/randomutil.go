package randomutil

import (
	"math/rand"
	"time"
)

type RandomGenerator interface {
	GenerateInt63() int64
	Intn(n int) int
}

const letters = "abcdefghijklmnopqrstuvwxyz"

type RandomNumberGenerator struct{}

func (RandomNumberGenerator) GenerateInt63() int64 {
	return rand.Int63()
}

func (r RandomNumberGenerator) Intn(n int) int {
	return rand.Intn(n)
}

func RandomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

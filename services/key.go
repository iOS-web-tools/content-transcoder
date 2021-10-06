package services

import (
	"crypto/sha1"
	"fmt"

	"github.com/urfave/cli"
)

type Key struct {
	key string
}

func NewKey(c *cli.Context) *Key {
	key := fmt.Sprintf("%x", sha1.Sum([]byte(c.String(KeyPrefixFlag)+c.String(infoHashFlag)+c.String(originPathFlag))))
	return &Key{
		key: key,
	}
}

func (s *Key) Get() string {
	return s.key
}

package memongo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tryvium-travels/memongo"
)

func TestRandomDatabase(t *testing.T) {
	s := memongo.RandomDatabase()

	assert.Len(t, s, memongo.DBNameLen)

	dbNameRunes := []rune(memongo.DBNameChars)
	for _, c := range s {
		assert.Contains(t, dbNameRunes, c)
	}
}

func TestRandomDatabaseEntropy(t *testing.T) {
	seen := map[string]bool{}

	for i := 0; i < 1000; i++ {
		s := memongo.RandomDatabase()
		assert.False(t, seen[s])

		seen[s] = true
	}
}

package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/server/model"
)

func TestAdmins(t *testing.T) {
	a := NewAdmins([]string{"woodpecker-ci"})
	assert.True(t, a.IsAdmin(&model.User{Login: "woodpecker-ci"}))
	assert.False(t, a.IsAdmin(&model.User{Login: "not-woodpecker-ci"}))
	empty := NewAdmins([]string{})
	assert.False(t, empty.IsAdmin(&model.User{Login: "woodpecker-ci"}))
	assert.False(t, empty.IsAdmin(&model.User{Login: "not-woodpecker-ci"}))
}

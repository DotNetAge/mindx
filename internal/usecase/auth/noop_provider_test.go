package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoopProvider_Name(t *testing.T) {
	p := NewNoopProvider()
	assert.Equal(t, "noop", p.Name())
}

func TestNoopProvider_Enabled(t *testing.T) {
	p := NewNoopProvider()
	assert.False(t, p.Enabled())
}

func TestNoopProvider_Middleware(t *testing.T) {
	p := NewNoopProvider()
	assert.Nil(t, p.Middleware())
}

func TestNoopProvider_PublicPaths(t *testing.T) {
	p := NewNoopProvider()
	assert.Nil(t, p.PublicPaths())
}

package arithmetic

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestModExp(t *testing.T) {
	result := ModExp(2, 13, 7)

	assert.Equal(t, 2, result)
}

package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPrettyTime(t *testing.T) {
	assert.Equal(t, "5h", prettyTime(5*time.Hour))
	assert.Equal(t, "6h", prettyTime(6*time.Hour))
	assert.Equal(t, "6h10m", prettyTime(6*time.Hour+10*time.Minute))
	assert.Equal(t, "6h10m54s", prettyTime(6*time.Hour+10*time.Minute+54*time.Second))
	assert.Equal(t, "9d6h10m54s", prettyTime(9*24*time.Hour+6*time.Hour+10*time.Minute+54*time.Second))
	assert.Equal(t, "4y43m12s", prettyTime(4*365.25*24*time.Hour))
	assert.Equal(t, "4y", prettyTime(126227808*time.Second))
	assert.Equal(t, "55y10d", prettyTime(1736496360*time.Second))
}

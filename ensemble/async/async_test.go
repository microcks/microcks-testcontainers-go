package async

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	goodHostPort = "host:8080"
	badHostPort  = "host"
)

func TestMustConvertPortToInt(t *testing.T) {
	bp, err := convertPortToInt(goodHostPort)
	require.NoError(t, err)
	require.Equal(t, 8080, bp)

	gp, err := convertPortToInt(badHostPort)
	require.Error(t, err)
	require.Equal(t, 0, gp)
}

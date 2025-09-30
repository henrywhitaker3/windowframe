package http

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestItReplacesEchoParamsFromURL(t *testing.T) {
	tcs := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "replaces simple id",
			input:    "/bongo/:id",
			expected: "/bongo/{id}",
		},
		{
			name:     "replaces param with underscores",
			input:    "/bongo/:some_id",
			expected: "/bongo/{some_id}",
		},
	}

	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			out := replaceParams(c.input)
			require.Equal(t, c.expected, out)
		})
	}
}

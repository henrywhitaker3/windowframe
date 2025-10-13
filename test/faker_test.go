package test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestItGeneratesSentencesOfGivenLength(t *testing.T) {
	tcs := []struct {
		name string
		size int
	}{
		{
			name: "generates sentence with 2 words",
			size: 2,
		},
		{
			name: "generates sentence with 20 words",
			size: 20,
		},
	}

	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			out := Sentence(c.size)
			require.Equal(t, c.size, len(strings.Split(out, " ")))
		})
	}
}

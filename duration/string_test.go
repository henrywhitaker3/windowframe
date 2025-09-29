package common

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestItUnmarshalsStringDurations(t *testing.T) {
	tcs := []struct {
		name     string
		input    string
		expected time.Duration
		errors   bool
	}{
		{
			name:     "parses milliseconds",
			input:    `"250ms"`,
			expected: time.Millisecond * 250,
			errors:   false,
		},
		{
			name:     "parses seconds",
			input:    `"250s"`,
			expected: time.Second * 250,
			errors:   false,
		},
		{
			name:     "parses hours",
			input:    `"5h"`,
			expected: time.Hour * 5,
			errors:   false,
		},
		{
			name:     "parses days",
			input:    `"2d"`,
			expected: time.Hour * 48,
			errors:   false,
		},
		{
			name:     "parses days and hours",
			input:    `"2d4h"`,
			expected: time.Hour * 52,
			errors:   false,
		},
	}

	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			var out StringDuration
			err := json.Unmarshal([]byte(c.input), &out)
			if c.errors {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, c.expected, out.Duration())
		})
	}
}

func TestItMarshallsStringDuration(t *testing.T) {
	dur := StringDuration(time.Millisecond * 250)
	out, err := json.Marshal(dur)
	require.Nil(t, err)
	require.Equal(t, "\"250ms\"", string(out))
}

func TestItMarshallsText(t *testing.T) {
	tcs := []struct {
		name     string
		dur      StringDuration
		expected string
	}{
		{
			name:     "marhsalls milliseconds",
			dur:      StringDuration(time.Millisecond * 250),
			expected: "250ms",
		},
		{
			name:     "marhsalls seconds",
			dur:      StringDuration(time.Second * 25),
			expected: "25s",
		},
		{
			name:     "marhsalls minutes",
			dur:      StringDuration(time.Minute * 25),
			expected: "25m0s",
		},
		{
			name:     "marhsalls hours",
			dur:      StringDuration(time.Hour * 25),
			expected: "1d1h0m0s",
		},
	}

	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			out, err := c.dur.MarshalText()
			require.Nil(t, err)
			require.Equal(t, c.expected, string(out))
		})
	}
}

func TestItUnmarshalsText(t *testing.T) {
	tcs := []struct {
		name   string
		input  string
		output time.Duration
	}{
		{
			name:   "unmarhsals milliseconds",
			input:  "250ms",
			output: time.Millisecond * 250,
		},
		{
			name:   "unmarhsals seconds",
			input:  "250s",
			output: time.Second * 250,
		},
		{
			name:   "unmarhsals minutes",
			input:  "250m",
			output: time.Minute * 250,
		},
		{
			name:   "unmarhsals hours",
			input:  "250h",
			output: time.Hour * 250,
		},
		{
			name:   "unmarhsals days",
			input:  "250d",
			output: time.Hour * 24 * 250,
		},
	}

	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			var out StringDuration
			require.Nil(t, out.UnmarshalText([]byte(c.input)))
			require.Equal(t, c.output, out.Duration())
		})
	}
}

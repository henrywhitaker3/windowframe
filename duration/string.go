// Package duration
package common

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type StringDuration time.Duration

var (
	hoursRegex = regexp.MustCompile(`[0-9]+h`)
	daysRegex  = regexp.MustCompile(`[0-9]+d`)
)

func ParseStringDuration(in string) (StringDuration, error) {
	// Replace any days in the duration with hours
	matches := daysRegex.FindAllString(in, -1)
	for _, m := range matches {
		// Ignore the error, we know they're valid numbers
		days, _ := strconv.Atoi(strings.ReplaceAll(m, "d", ""))
		in = strings.ReplaceAll(in, m, fmt.Sprintf("%dh", days*24))
	}
	dur, err := time.ParseDuration(in)
	if err != nil {
		return StringDuration(0), fmt.Errorf("failed to parse string duration: %w", err)
	}
	return StringDuration(dur), nil
}

func (s StringDuration) Duration() time.Duration {
	return time.Duration(s)
}

func (s StringDuration) String() string {
	out := s.Duration().String()
	matches := hoursRegex.FindAllString(out, -1)
	for _, m := range matches {
		// Ignore the error as we know it's a valid int
		hours, _ := strconv.Atoi(strings.ReplaceAll(m, "h", ""))
		days := 0
		for hours >= 24 {
			days++
			hours = hours - 24
		}
		new := fmt.Sprintf("%dd%dh", days, hours)
		out = strings.ReplaceAll(out, m, new)
	}
	return out
}

func (s StringDuration) MarshalJSON() ([]byte, error) {
	return fmt.Appendf(nil, "\"%s\"", s.String()), nil
}

func (s *StringDuration) UnmarshalJSON(raw []byte) error {
	var str string
	if err := json.Unmarshal(raw, &str); err != nil {
		return err
	}
	parsed, err := ParseStringDuration(str)
	if err != nil {
		return fmt.Errorf("failed to parse duration: %w", err)
	}
	*s = parsed
	return nil
}

func (s StringDuration) MarshalText() ([]byte, error) {
	str := s.String()
	return []byte(str), nil
}

func (s *StringDuration) UnmarshalText(text []byte) error {
	dur, err := ParseStringDuration(string(text))
	if err != nil {
		return fmt.Errorf("failed to parse string duration: %w", err)
	}
	*s = dur
	return nil
}

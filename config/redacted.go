package config

const redactedValue = "[REDACTED]"

// RedactedString holds sensitive configuration values. Its serialized form is
// always redacted to prevent accidental disclosure in logs or other output.
type RedactedString string

func (RedactedString) MarshalText() ([]byte, error) {
	return []byte(redactedValue), nil
}

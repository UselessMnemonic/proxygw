package config

import (
	"fmt"
	"math"
	"time"
)

// TTL is a timeout value represented in seconds.
type TTL uint32

const (
	// TTLMax is the largest representable TTL value.
	TTLMax = TTL(math.MaxUint32)
	// TTLMin is the smallest representable TTL value.
	TTLMin = TTL(0)
)

const (
	ttlMaxDuration = time.Duration(math.MaxUint32) * time.Second
	ttlMinDuration = time.Duration(0)
)

// ParseTTL parses a duration string and converts it to whole seconds.
func ParseTTL(text string) (TTL, error) {
	result, err := time.ParseDuration(text)
	if err != nil {
		return 0, err
	}
	if !(ttlMinDuration <= result && result <= ttlMaxDuration) {
		return 0, fmt.Errorf("invalid TTL %q", text)
	}
	return TTL(result / time.Second), err
}

// Seconds returns the TTL value as seconds.
func (t TTL) Seconds() uint32 {
	return uint32(t)
}

// ToDuration converts the TTL value to a time.Duration.
func (t TTL) ToDuration() time.Duration {
	return time.Duration(t) * time.Second
}

func (t TTL) String() string {
	return fmt.Sprintf("%ds", t.Seconds())
}

func (t TTL) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t *TTL) UnmarshalText(text []byte) error {
	ttlString := string(text)
	result, err := ParseTTL(ttlString)
	if err != nil {
		return err
	}
	*t = result
	return nil
}

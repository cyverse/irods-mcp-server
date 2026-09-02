package common

import (
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

const (
	KiloBytes int64 = 1024
	MegaBytes int64 = KiloBytes * 1024
	GigaBytes int64 = MegaBytes * 1024
	TeraBytes int64 = GigaBytes * 1024

	Minute int = 60
	Hour   int = Minute * 60
	Day    int = Hour * 24
)

func ParseSize(size string) (int64, error) {
	size = strings.TrimSpace(size)
	size = strings.ToUpper(size)
	size = strings.TrimSuffix(size, "B")

	if len(size) == 0 {
		return 0, nil
	}

	multiplier := int64(1)
	numStr := size

	switch size[len(size)-1] {
	case 'K':
		multiplier = KiloBytes
		numStr = size[:len(size)-1]
	case 'M':
		multiplier = MegaBytes
		numStr = size[:len(size)-1]
	case 'G':
		multiplier = GigaBytes
		numStr = size[:len(size)-1]
	case 'T':
		multiplier = TeraBytes
		numStr = size[:len(size)-1]
	}

	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to convert string %q to int", size)
	}
	return n * multiplier, nil
}

func ParseTime(t string) (int, error) {
	t = strings.TrimSpace(t)
	t = strings.ToUpper(t)

	if len(t) == 0 {
		return 0, nil
	}

	multiplier := int64(1)
	numStr := t

	switch t[len(t)-1] {
	case 'S':
		numStr = t[:len(t)-1]
	case 'M':
		multiplier = int64(Minute)
		numStr = t[:len(t)-1]
	case 'H':
		multiplier = int64(Hour)
		numStr = t[:len(t)-1]
	case 'D':
		multiplier = int64(Day)
		numStr = t[:len(t)-1]
	}

	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to convert string %q to int", t)
	}
	return int(n * multiplier), nil
}

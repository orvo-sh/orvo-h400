package util

import "time"

func NanoToTime(nanos uint64) time.Time {
	if nanos == 0 {
		return time.Time{}
	}
	return time.Unix(0, int64(nanos)).UTC()
}

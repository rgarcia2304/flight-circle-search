package cache

import "time"

const defaultTTL = time.Hour

func FareKey(origin, destination, date string) string {
	return "fare:" + origin + ":" + destination + ":" + date
}

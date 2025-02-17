package bone

import "time"

func TimeSec() int64 {
	return time.Now().Unix()
}

// Formats timestamp to a date.
func DateSec(sec int64, format string) string {
	return time.Unix(sec, 0).Format(format)
}

func TimeSleep(duration int64) {
	time.Sleep(time.Duration(duration * 1000))
}

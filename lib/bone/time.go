package bone

import "time"

func Utc() int64 {
	return time.Now().Unix()
}

// Formats timestamp to a date.
func Date_Sec(sec int, format string) string {
	return time.Unix(int64(sec), 0).Format(format)
}

func Sleep_Sec(duration_sec int64) {
	time.Sleep(time.Duration(duration_sec) * time.Second)
}

func Sleep_Ms(duration_ms int64) {
	time.Sleep(time.Duration(duration_ms) * time.Millisecond)
}

// Time is in milliseconds, unless other is clearly specified.
package bone

import "time"

func Utc() int64 {
	return time.Now().Unix()
}

// Formats timestamp to a date.
func Date_Sec(sec int, format string) string {
	return time.Unix(int64(sec), 0).Format(format)
}

func Sleep_Ms(duration_ms int64) {
	time.Sleep(time.Duration(duration_ms) * time.Millisecond)
}

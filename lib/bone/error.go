package bone

import (
	"fmt"
)

func Error(message string, args ...any) error {
	return fmt.Errorf(message+"\n", args...)
}

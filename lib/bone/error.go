package bone

import (
	"fmt"
)

func New_Error(message string, args ...any) error {
	return fmt.Errorf(message+"\n", args...)
}

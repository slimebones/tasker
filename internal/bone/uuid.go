package bone

import (
	"strings"

	"github.com/google/uuid"
)

func Uuid() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

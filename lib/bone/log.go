package bone

import "fmt"

// @todo use loguru-like sinks

func Log(message string, args ...any) {
	fmt.Printf(message+"\n", args...)
}

func Log_Error(message string, args ...any) {
	const RED = "\033[91m"
	const RESET = "\033[0m"
	message = fmt.Sprintf(message, args...)
	fmt.Printf("%sERROR%s: %s\n", RED, RESET, message)
}

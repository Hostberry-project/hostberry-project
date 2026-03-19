package main

import (
	"time"

	"hostberry/internal/utils"
)

// Wrappers para mantener compatibilidad con el resto del código `package main`.

func createDefaultAdmin() {
	utils.CreateDefaultAdmin()
}

func executeCommand(cmd string) (string, error) {
	return utils.ExecuteCommand(cmd)
}

func executeCommandWithTimeout(cmd string, timeout time.Duration) (string, error) {
	return utils.ExecuteCommandWithTimeout(cmd, timeout)
}

func filterSudoErrors(output []byte) string {
	return utils.FilterSudoErrors(output)
}

func filterSudoErrorString(output string) string {
	return utils.FilterSudoErrorString(output)
}

func strconvAtoiSafe(s string) (int, error) {
	return utils.StrconvAtoiSafe(s)
}

func mapActiveStatus(status string) string {
	return utils.MapActiveStatus(status)
}

func mapBoolStatus(v string) string {
	return utils.MapBoolStatus(v)
}


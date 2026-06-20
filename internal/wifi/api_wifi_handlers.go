package wifi

import (
	"os/exec"

	"hostberry/internal/utils"
)

// Wrappers para compatibilidad con lógica original desde `package main`.
func execCommand(cmd string) *exec.Cmd { return utils.ExecCommand(cmd) }

func executeCommand(cmd string) (string, error) { return utils.ExecuteCommand(cmd) }

func filterSudoErrors(output []byte) string { return utils.FilterSudoErrors(output) }

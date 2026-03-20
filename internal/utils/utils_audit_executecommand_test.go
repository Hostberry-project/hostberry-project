package utils

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("Could not find repo root starting from %s", wd)
		}
		dir = parent
	}
}

func allowedCommandsForAudit() []string {
	// Copia (mecánica) de la lista allowlist usada en ExecuteCommandWithTimeout.
	return []string{
		// Nota seguridad: no permitimos shells como "sh" o "bash" desde executeCommand.
		"hostname", "hostnamectl", "uname", "cat", "grep", "awk", "sed", "cut", "head", "tail",
		"top", "free", "df", "nproc",
		"iwlist", "nmcli", "iw",
		"ip", "wg", "wg-quick", "systemctl", "pgrep",
		"reboot", "shutdown", "poweroff",
		"rfkill", "ifconfig", "iwconfig",
		"hostapd", "hostapd_cli", "dnsmasq", "iptables", "iptables-save", "netfilter-persistent", "sysctl", "tee", "cp", "mkdir", "echo", "chmod",
		"dhclient", "udhcpc", "wpa_supplicant", "wpa_cli", "pkill", "killall",
		"true",
	}
}

func TestAudit_ExecuteCommandStringLiterals_AreAllowlisted(t *testing.T) {
	root := findRepoRoot(t)
	allowed := allowedCommandsForAudit()

	failed := 0
	var failExamples []string

	parseFile := func(path string) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			// Ignoramos archivos que no parseen (poco frecuente; no deben bloquear el audit).
			return
		}

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Detectar llamadas: ExecuteCommand("...") / executeCommand("...") / utils.ExecuteCommand("...")
			isRelevant := false
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				if fn.Name == "ExecuteCommand" || fn.Name == "executeCommand" {
					isRelevant = true
				}
			case *ast.SelectorExpr:
				if fn.Sel != nil && fn.Sel.Name == "ExecuteCommand" {
					isRelevant = true
				}
			}
			if !isRelevant {
				return true
			}

			if len(call.Args) == 0 {
				return true
			}

			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				// Solo auditar literales: dinámicos (fmt.Sprintf/variables) pueden necesitar evaluación.
				return true
			}

			cmd, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if cmd == "" {
				return true
			}

			if err := validateShellCommandAllowList(cmd, allowed); err != nil {
				failed++
				failExamples = append(failExamples, path+": "+err.Error()+" cmd="+cmd)
			}

			return true
		})
	}

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		parseFile(path)
		return nil
	})

	if failed > 0 {
		t.Fatalf("Audit failed: %d executeCommand/ExecuteCommand string literals are NOT allowlisted. Examples: %v", failed, failExamples)
	}
}


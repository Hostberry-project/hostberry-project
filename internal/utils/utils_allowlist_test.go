package utils

import (
	"os/exec"
	"testing"
)

func TestValidateShellCommandAllowList_AllowedBasics(t *testing.T) {
	allowed := []string{"ip", "grep", "echo", "nmcli"}

	cases := []struct {
		name string
		cmd  string
		ok   bool
	}{
		{"simple_ip", "ip route", true},
		{"pipe", "ip route | grep foo", true},
		{"or", "ip route || grep foo", true},
		{"and", "ip route && grep foo", true},
		{"quoted_operator_is_literal", "ip route '&& rm -rf /'", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateShellCommandAllowList(tc.cmd, allowed)
			if tc.ok && err != nil {
				t.Fatalf("expected allow, got err=%v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected deny, got allow")
			}
		})
	}
}

func TestValidateShellCommandAllowList_Denies(t *testing.T) {
	allowed := []string{"ip", "grep", "echo"}

	cases := []struct {
		name string
		cmd  string
	}{
		{"not_allowed_base", "rm -rf /"},
		{"semicolon", "ip route; rm -rf /"},
		{"backticks", "ip route `id`"},
		{"unbalanced_single_quote", "ip route 'grep foo"},
		// Nota: & suelto no es operador en el validator (ej. 2>&1),
		// por eso no se prohíbe en este test.
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateShellCommandAllowList(tc.cmd, allowed)
			if err == nil {
				t.Fatalf("expected deny, got allow")
			}
			if err != exec.ErrNotFound {
				// validateShellCommandAllowList actualmente usa exec.ErrNotFound como señal.
				t.Fatalf("expected exec.ErrNotFound, got %v", err)
			}
		})
	}
}


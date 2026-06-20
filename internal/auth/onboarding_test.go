package auth

import (
	"testing"

	"hostberry/internal/models"
)

func TestPostLoginWebPath(t *testing.T) {
	wizardOnly := &models.User{SetupWizardCompleted: false, FirstLoginCompleted: false}
	if p := PostLoginWebPath(wizardOnly); p != "/setup-wizard" {
		t.Fatalf("wizard only: got %q", p)
	}

	firstLogin := &models.User{SetupWizardCompleted: true, FirstLoginCompleted: false}
	if p := PostLoginWebPath(firstLogin); p != "/first-login" {
		t.Fatalf("first login: got %q", p)
	}

	done := &models.User{SetupWizardCompleted: true, FirstLoginCompleted: true}
	if p := PostLoginWebPath(done); p != "/dashboard" {
		t.Fatalf("done: got %q", p)
	}
}

func TestIsPasswordChangeRequiredNeedsWizardFirst(t *testing.T) {
	user := &models.User{SetupWizardCompleted: false, FirstLoginCompleted: false}
	if IsPasswordChangeRequired(user) {
		t.Fatal("password change should not be required before wizard")
	}
}

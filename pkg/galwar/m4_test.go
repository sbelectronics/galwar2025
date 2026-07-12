package galwar

import (
	"testing"
)

func TestRegisterPlayer(t *testing.T) {
	u := NewUniverse()
	u.Generate(50)
	u.SeedDefaultConfig()

	p, err := u.RegisterPlayer("Scott", "scott@example.com", "sub-1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if p.GoogleSub != "sub-1" {
		t.Errorf("GoogleSub = %q; want sub-1", p.GoogleSub)
	}
	if p.GetQuantity(TURNS) != 250 {
		t.Errorf("new player turns = %d; want 250", p.GetQuantity(TURNS))
	}

	// moderation applies
	if _, err := u.RegisterPlayer("fuck", "x@example.com", "sub-2"); err == nil {
		t.Errorf("profane handle accepted")
	}
	if _, err := u.RegisterPlayer("Sysop", "x@example.com", "sub-2"); err == nil {
		t.Errorf("reserved handle accepted")
	}

	// uniqueness is on the normalized form: sc0tt == Scott
	if _, err := u.RegisterPlayer("sc0tt", "x@example.com", "sub-2"); err == nil {
		t.Errorf("normalized-duplicate handle accepted")
	}
	if _, err := u.RegisterPlayer("S c o t t", "x@example.com", "sub-2"); err == nil {
		t.Errorf("spaced-duplicate handle accepted")
	}

	// lookups
	if u.Players.GetBySub("sub-1") != p {
		t.Errorf("GetBySub failed")
	}
	if u.Players.GetByNormalizedName("SC0TT") != p {
		t.Errorf("GetByNormalizedName failed")
	}
	if u.Players.GetBySub("") != nil {
		t.Errorf("GetBySub(\"\") must not match empty-sub records")
	}
}

func TestTelnetPassword(t *testing.T) {
	u := NewUniverse()
	u.Generate(50)
	u.SeedDefaultConfig()
	p, err := u.RegisterPlayer("Passy", "p@example.com", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if p.CheckTelnetPassword("anything") {
		t.Errorf("password check passed with no password set")
	}
	if err := u.SetTelnetPassword(p, "short"); err == nil {
		t.Errorf("5-char password accepted")
	}
	if err := u.SetTelnetPassword(p, "opensesame"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	if !p.CheckTelnetPassword("opensesame") {
		t.Errorf("correct password rejected")
	}
	if p.CheckTelnetPassword("wrong") {
		t.Errorf("wrong password accepted")
	}
}

package galwar

import (
	"strings"
	"time"

	"github.com/sbelectronics/galwar/pkg/moderation"
)

// Sysop tooling (PLAN.md section 8, layer 6): player reports, an audit trail,
// and the admin actions - ban, force-rename - that act on them. Admins are
// designated by the config "admins" key (a comma-separated list of emails),
// so the operator bootstraps the first admin out-of-band and the rest is
// in-game.

type Report struct {
	Reporter string // reporter handle
	Target   string // reported handle
	Reason   string
	At       int64
	Resolved bool
}

type AuditEntry struct {
	At     int64
	Actor  string
	Action string
	Detail string
}

const maxAuditEntries = 5000

// IsAdmin reports whether a player is a sysop.
func (u *UniverseType) IsAdmin(p *Player) bool {
	if p == nil || p.Email == "" {
		return false
	}
	for _, a := range strings.Split(u.ConfigString("admins", ""), ",") {
		if strings.EqualFold(strings.TrimSpace(a), p.Email) {
			return true
		}
	}
	return false
}

// AddAudit appends an entry to the security/admin audit trail.
func (u *UniverseType) AddAudit(now int64, actor, action, detail string) {
	u.Audit = append(u.Audit, &AuditEntry{At: now, Actor: actor, Action: action, Detail: detail})
	if len(u.Audit) > maxAuditEntries {
		u.Audit = u.Audit[len(u.Audit)-maxAuditEntries:]
	}
	u.MarkDirty()
}

func (u *UniverseType) trimAudit() {
	if len(u.Audit) > maxAuditEntries {
		u.Audit = u.Audit[len(u.Audit)-maxAuditEntries:]
	}
}

// FileReport records a player's report of another player.
func (u *UniverseType) FileReport(reporter *Player, targetHandle, reason string) error {
	if reporter == nil {
		return NewGameError(ErrUnknown, "You must be signed in to file a report.")
	}
	targetHandle = strings.TrimSpace(targetHandle)
	reason = strings.TrimSpace(reason)
	target := u.Players.GetByNormalizedName(targetHandle)
	if target == nil {
		return NewGameError(ErrNotFound, "No trader by that handle.")
	}
	if target == reporter {
		return NewGameError(ErrUnknown, "You can't report yourself.")
	}
	now := time.Now().Unix()
	u.Reports = append(u.Reports, &Report{
		Reporter: reporter.GetName(),
		Target:   target.GetName(),
		Reason:   reason,
		At:       now,
	})
	u.AddAudit(now, reporter.GetName(), "report", target.GetName()+": "+reason)
	u.MarkDirty()
	return nil
}

// OpenReports returns the unresolved reports.
func (u *UniverseType) OpenReports() []*Report {
	var out []*Report
	for _, r := range u.Reports {
		if !r.Resolved {
			out = append(out, r)
		}
	}
	return out
}

// ResolveReports marks every report against a handle resolved.
func (u *UniverseType) ResolveReports(targetHandle string) {
	norm := moderation.Normalize(targetHandle)
	for _, r := range u.Reports {
		if moderation.Normalize(r.Target) == norm {
			r.Resolved = true
		}
	}
	u.MarkDirty()
}

// SetBanned bans or unbans a player by handle. Admin action: the engine
// verifies the caller is a sysop rather than trusting the UI to gate it.
func (u *UniverseType) SetBanned(admin *Player, targetHandle string, banned bool) error {
	if admin == nil || !u.IsAdmin(admin) {
		return NewGameError(ErrNotOwner, "Access denied.")
	}
	target := u.Players.GetByNormalizedName(targetHandle)
	if target == nil {
		return NewGameError(ErrNotFound, "No trader by that handle.")
	}
	if u.IsAdmin(target) {
		return NewGameError(ErrNotOwner, "You can't ban another sysop.")
	}
	target.Banned = banned
	action := "ban"
	if !banned {
		action = "unban"
	}
	u.AddAudit(time.Now().Unix(), admin.GetName(), action, target.GetName())
	u.MarkDirty()
	return nil
}

// ForceRename changes a player's handle to a moderated new one (admin action).
// The new handle must pass the moderation pipeline and be unique. The engine
// verifies the caller is a sysop rather than trusting the UI to gate it.
func (u *UniverseType) ForceRename(admin *Player, targetHandle, newName string) error {
	if admin == nil || !u.IsAdmin(admin) {
		return NewGameError(ErrNotOwner, "Access denied.")
	}
	target := u.Players.GetByNormalizedName(targetHandle)
	if target == nil {
		return NewGameError(ErrNotFound, "No trader by that handle.")
	}
	newName = strings.TrimSpace(newName)
	if err := moderation.CheckName(newName); err != nil {
		return NewGameError(ErrInvalidName, err.Error())
	}
	norm := moderation.Normalize(newName)
	for _, p := range u.Players.Players {
		if p != target && moderation.Normalize(p.Name) == norm {
			return NewGameError(ErrAlreadyExists, "That handle is already taken.")
		}
	}
	old := target.GetName()
	target.Name = newName
	u.ResolveReports(old)
	u.AddAudit(time.Now().Unix(), admin.GetName(), "rename", old+" -> "+newName)
	u.MarkDirty()
	return nil
}

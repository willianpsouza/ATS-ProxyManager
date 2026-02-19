package domain

type UserRole string

const (
	RoleRoot    UserRole = "root"
	RoleAdmin   UserRole = "admin"
	RoleRegular UserRole = "regular"
)

func (r UserRole) IsValid() bool {
	switch r {
	case RoleRoot, RoleAdmin, RoleRegular:
		return true
	}
	return false
}

// CanCreate checks if this role can create users with the target role.
func (r UserRole) CanCreate(target UserRole) bool {
	switch r {
	case RoleRoot:
		return target == RoleAdmin || target == RoleRegular
	case RoleAdmin:
		return target == RoleRegular
	default:
		return false
	}
}

type ConfigStatus string

const (
	StatusDraft           ConfigStatus = "draft"
	StatusPendingApproval ConfigStatus = "pending_approval"
	StatusApproved        ConfigStatus = "approved"
	StatusActive          ConfigStatus = "active"
)

func (s ConfigStatus) IsValid() bool {
	switch s {
	case StatusDraft, StatusPendingApproval, StatusApproved, StatusActive:
		return true
	}
	return false
}

type RuleAction string

const (
	ActionDirect RuleAction = "direct"
	ActionParent RuleAction = "parent"
)

func (a RuleAction) IsValid() bool {
	switch a {
	case ActionDirect, ActionParent:
		return true
	}
	return false
}

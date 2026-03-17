package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRole defines the access level of a user within Fluidify Regen.
type UserRole string

const (
	UserRoleAdmin  UserRole = "admin"
	UserRoleMember UserRole = "member"
	UserRoleViewer UserRole = "viewer"
)

// User represents a person or AI agent in Fluidify Regen.
// Human users authenticate via SAML SSO (auth_source='saml') or local credentials (auth_source='local').
// AI agent users have auth_source='ai', a non-null AgentType, and no password.
// SAML users have no password hash; local users have no SAML subject.
// Human users are provisioned automatically on first login (JIT provisioning for SAML)
// or by an admin (for local auth). AI agent users are seeded on startup.
type User struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email string   `gorm:"type:varchar(255);not null;uniqueIndex"          json:"email"`
	Name  string   `gorm:"type:varchar(255);not null;default:''"           json:"name"`

	// SAMLSubject is the NameID from the SAML assertion — immutable after first login.
	// Nullable so that locally-authenticated users can be stored without a SAML subject.
	SAMLSubject   *string `gorm:"type:varchar(500);uniqueIndex;column:saml_subject" json:"-"`
	SAMLIDPIssuer string  `gorm:"type:varchar(500);not null;default:'';column:saml_idp_issuer" json:"-"`

	// Local auth fields — only set for auth_source='local'.
	PasswordHash *string `gorm:"type:text;column:password_hash" json:"-"`
	AuthSource   string  `gorm:"type:varchar(20);not null;default:'saml';column:auth_source" json:"-"`

	// AgentType identifies AI agent accounts. NULL for all human users.
	// Valid values: "postmortem", "triage", "comms", "oncall", "commander"
	AgentType *string `gorm:"type:varchar(50);column:agent_type" json:"agent_type,omitempty"`

	// Active controls whether this user (or agent) can operate.
	// Defaults to true for all existing rows via migration.
	Active bool `gorm:"not null;default:true;column:active" json:"active"`

	// SlackUserID is the Slack member ID (e.g. U0AJLLY3678).
	// Set automatically when user is imported from Slack, or manually by admin.
	// Used to invite on-call responders to incident channels and send DMs.
	SlackUserID *string `gorm:"type:varchar(20);column:slack_user_id" json:"slack_user_id,omitempty"`

	// TeamsUserID is the Azure AD Object ID (e.g. 29dcb621-b60b-4b3d-aa41-...).
	// Set automatically when user is imported from Teams, or manually by admin.
	// Used to send proactive Adaptive Card DMs via Bot Framework.
	TeamsUserID *string `gorm:"type:varchar(255);column:teams_user_id" json:"teams_user_id,omitempty"`

	Role        UserRole   `gorm:"type:varchar(50);not null;default:'member'" json:"role"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (User) TableName() string { return "users" }

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

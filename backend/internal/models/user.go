package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRole defines the access level of a user within OpenIncident.
type UserRole string

const (
	UserRoleAdmin  UserRole = "admin"
	UserRoleMember UserRole = "member"
	UserRoleViewer UserRole = "viewer"
)

// User represents a person authenticated via SAML SSO or local credentials.
// SAML users have no password hash; local users have no SAML subject.
// Users are provisioned automatically on first login (JIT provisioning for SAML)
// or by an admin (for local auth).
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

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

// User represents a person authenticated via SAML SSO.
// No passwords are stored — authentication is fully delegated to the IdP.
// Users are provisioned automatically on first login (JIT provisioning).
type User struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email string   `gorm:"type:varchar(255);not null;uniqueIndex"          json:"email"`
	Name  string   `gorm:"type:varchar(255);not null;default:''"           json:"name"`

	// SAMLSubject is the NameID from the SAML assertion — immutable after first login.
	SAMLSubject   string `gorm:"type:varchar(500);not null;uniqueIndex;column:saml_subject" json:"-"`
	SAMLIDPIssuer string `gorm:"type:varchar(500);not null;default:'';column:saml_idp_issuer" json:"-"`

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

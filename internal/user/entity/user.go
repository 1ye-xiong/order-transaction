package entity

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrAccountAlreadyExists = errors.New("account already exists")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrInvalidUser          = errors.New("invalid user")
)

// User 是 user 服务使用的用户聚合。
type User struct {
	ID           string
	Account      string
	DisplayName  string
	Role         string
	PasswordHash string
	PasswordSalt string
	Sex          bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// 支持的业务角色；身份由 JWT 携带，禁止通过请求参数冒充。
const (
	RoleBuyer     = "buyer"
	RoleOps       = "ops"
	RoleWarehouse = "warehouse"
	RoleCS        = "cs"
)

var validRoles = map[string]bool{RoleBuyer: true, RoleOps: true, RoleWarehouse: true, RoleCS: true}

// NewUser 校验输入，并创建带规范化账号的用户聚合。
func NewUser(account string, displayName string, passwordHash string, role string) (*User, error) {
	account = NormalizeAccount(account)
	displayName = strings.TrimSpace(displayName)
	if account == "" || displayName == "" || passwordHash == "" {
		return nil, ErrInvalidUser
	}
	if role == "" {
		role = RoleBuyer
	}
	if !validRoles[role] {
		return nil, ErrInvalidUser
	}

	now := time.Now().UTC()
	return &User{
		Account:      account,
		DisplayName:  displayName,
		Role:         role,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// NormalizeAccount 去除账号首尾空白并转小写，用于唯一查询。
func NormalizeAccount(account string) string {
	return strings.TrimSpace(strings.ToLower(account))
}

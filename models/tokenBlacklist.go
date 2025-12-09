package models

import "time"

type TokenBlacklist struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	TokenHash string    `json:"token_hash" gorm:"size:512;uniqueIndex"`
	UserID    string    `json:"user_id" gorm:"index"`
	Reason    string    `json:"reason"` // 撤销原因: logout, security, admin
	RevokedAt time.Time `json:"revoked_at"`
	ExpiresAt time.Time `json:"expires_at"` // 可选，自动过期时间
}
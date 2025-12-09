package models

import (
	"time"

	"gorm.io/gorm"
)

type ChatRoom struct {
	ID          string         `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"size:191"`                                 // 房间名称
	Description string         `json:"description"`                              // 房间描述
	Type        string         `json:"type"`                                     // 房间类型：private, group, channel
	CreatorID   string         `json:"creator_id" gorm:"index"`                  // 创建者ID
	Avatar      string         `json:"avatar"`                                   // 房间头像
	IsPublic    bool           `json:"is_public"`                                // 是否公开
	MaxMembers  int            `json:"max_members"`                              // 最大成员数
	InviteCode  string         `json:"invite_code,omitempty" gorm:"size:191;uniqueIndex"` // 邀请码
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"` // 软删除

	// 关联
	Members []*RoomMember `json:"members,omitempty" gorm:"foreignKey:RoomID"`
}

type RoomMember struct {
	ID       string    `json:"id" gorm:"primaryKey"`
	RoomID   string    `json:"room_id" gorm:"index:idx_room_user,unique"`
	UserID   string    `json:"user_id" gorm:"index:idx_room_user,unique"`
	Role     string    `json:"role"` // owner, admin, member
	JoinedAt time.Time `json:"joined_at"`
	Nickname string    `json:"nickname"` // 在房间内的昵称

	// 关联
	User *User     `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Room *ChatRoom `json:"room,omitempty" gorm:"foreignKey:RoomID"`
}

type RoomStats struct {
	TotalMembers    int64     `json:"total_members"`
	TotalMessages   int64     `json:"total_messages"`
	LastActivity    time.Time `json:"last_activity"`
	ActiveMembers   int64     `json:"active_members"`
	NewMembersToday int64     `json:"new_members_today"`
	MessagesToday   int64     `json:"messages_today"`
}

type ActiveRoom struct {
	RoomID        string    `json:"room_id"`
	RoomName      string    `json:"room_name"`
	MessageCount  int64     `json:"message_count"`
	LastMessageAt time.Time `json:"last_message_at"`
	ActiveUsers   int64     `json:"active_users"`
}

package models

import (
	"encoding/json"
	"time"
)

const (
	MsgTypeText        = "text"
	MsgTypeImage       = "image"
	MsgTypeFile        = "file"
	MsgTypeVoice       = "voice"  // 语音消息
	MsgTypeVideo       = "video"  // 视频消息
	MsgTypeSystem      = "system" // 系统消息
	MsgTypeRecall      = "recall" // 消息撤回
	MsgTypeRead        = "read"   // 已读回执
	MsgTypeCreateGroup = "create_group"
	MsgTypeJoinRoom    = "join_group"
	MsgTypeLeaveRoom   = "leave_room"
	MsgTypeOnlineList  = "online_list"
	MsgTypePing        = "ping"
)

const (
	MsgStatusSending   = "sending"   // 发送中
	MsgStatusSent      = "sent"      // 已发送
	MsgStatusDelivered = "delivered" // 已送达
	MsgStatusRead      = "read"      // 已读
	MsgStatusFailed    = "failed"    // 发送失败
)

const (
	RecipientUser      = "user"
	RecipientRoom      = "room"
	RecipientBroadcast = "broadcast"
)

type Message struct {
	ID          string          `json:"id" gorm:"primaryKey"`
	RoomID      string          `json:"room_id" gorm:"index"`         // 房间ID
	SenderID    string          `json:"sender_id" gorm:"index"`       // 发送者ID
	SenderName  string          `json:"sender_name"`                  // 发送者名称
	MessageType string          `json:"message_type"`                 // 消息类型
	Content     string          `json:"content"`                      // 文本内容
	Metadata    json.RawMessage `json:"metadata" gorm:"type:json"`    // 元数据（图片URL、文件信息等）
	Status      string          `json:"status" gorm:"default:'sent'"` // 消息状态
	CreatedAt   time.Time       `json:"created_at" gorm:"index"`      // 创建时间
	UpdatedAt   time.Time       `json:"updated_at"`                   // 更新时间

	// 新增字段用于支持消息分发
	RecipientType string   `json:"recipient_type"`                     // 接收者类型: user, room, broadcast
	To            []string `json:"to,omitempty" gorm:"-"`              // 接收者ID列表（用户或房间）

	// 关联查询字段（不存储在数据库）
	Sender *User `json:"sender,omitempty" gorm:"-"`
}

type MessageMetadata struct {
	// 文件消息
	FileName string `json:"file_name,omitempty"`
	FileSize int64  `json:"file_size,omitempty"`
	FileType string `json:"file_type,omitempty"`
	FileURL  string `json:"file_url,omitempty"`

	// 图片消息
	ImageURL    string `json:"image_url,omitempty"`
	ImageWidth  int    `json:"image_width,omitempty"`
	ImageHeight int    `json:"image_height,omitempty"`
	Thumbnail   string `json:"thumbnail,omitempty"`

	// 语音消息
	VoiceURL      string `json:"voice_url,omitempty"`
	VoiceDuration int    `json:"voice_duration,omitempty"`

	// 视频消息
	VideoURL      string `json:"video_url,omitempty"`
	VideoDuration int    `json:"video_duration,omitempty"`
	ThumbnailURL  string `json:"thumbnail_url,omitempty"`

	// 撤回消息
	RecallBy     string `json:"recall_by,omitempty"`     // 撤回者
	RecallAt     string `json:"recall_at,omitempty"`     // 撤回时间
	RecallReason string `json:"recall_reason,omitempty"` // 撤回原因

	// 其他元数据
	MentionedUsers []string `json:"mentioned_users,omitempty"` // @的用户
	ReplyTo        string   `json:"reply_to,omitempty"`        // 回复的消息ID
	ForwardFrom    string   `json:"forward_from,omitempty"`    // 转发的来源
}

// ReadReceipt 已读回执
type ReadReceipt struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	MessageID string    `json:"message_id" gorm:"index"`
	UserID    string    `json:"user_id" gorm:"index"`
	RoomID    string    `json:"room_id" gorm:"index"`
	ReadAt    time.Time `json:"read_at"`
}

// MessageQuery 消息查询参数
type MessageQuery struct {
	RoomID       string    `json:"room_id"`
	SenderID     string    `json:"sender_id,omitempty"`
	StartTime    time.Time `json:"start_time,omitempty"`
	EndTime      time.Time `json:"end_time,omitempty"`
	Keyword      string    `json:"keyword,omitempty"`
	MessageTypes []string  `json:"message_types,omitempty"`
	Page         int       `json:"page"`
	PageSize     int       `json:"page_size"`
	OrderBy      string    `json:"order_by"` // "asc" or "desc"
}

// MessageResponse 消息响应
type MessageResponse struct {
	Messages    []Message `json:"messages"`
	Total       int64     `json:"total"`
	CurrentPage int       `json:"current_page"`
	TotalPages  int       `json:"total_pages"`
	HasMore     bool      `json:"has_more"`
}

type MessageStats struct {
	TotalMessages int64 `json:"total_messages"`
	TextMessages  int64 `json:"text_messages"`
	ImageMessages int64 `json:"image_messages"`
	FileMessages  int64 `json:"file_messages"`
	VoiceMessages int64 `json:"voice_messages"`
	VideoMessages int64 `json:"video_messages"`
	AveragePerDay int64 `json:"average_per_day"`
	BusiestHour   int   `json:"busiest_hour"`
	ActiveUsers   int64 `json:"active_users"`
}

package models

import (
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	UserID    string          `json:"user_id"`
	Username  string          `json:"username"`
	Conn      *websocket.Conn `json:"-"`
	Send      chan Message    `json:"-"` // 发送消息通道
	OnlineAt  time.Time       `json:"online_at"`
	IsOnline  bool            `json:"is_online"`
	IP        string          `json:"ip,omitempty"`
	UserAgent string          `json:"user_agent,omitempty"`
}

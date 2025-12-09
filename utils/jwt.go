package utils

import (
	"encoding/json"
	"socket/config"
	"time"

	"github.com/golang-jwt/jwt"
)

// CustomClaims 自定义声明结构体
type CustomClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.StandardClaims
}

func GenerateToken(userID, username string) (string, error) {
	// 设置过期时间
	expirationTime := time.Now().Add(24 * time.Hour)

	// 创建声明
	claims := &CustomClaims{
		UserID:   userID,
		Username: username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	// 创建令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 签名并返回
	return token.SignedString([]byte(config.Conf.JWTSecret))
}

// ParseToken 解析JWT令牌，提取用户信息
func ParseToken(tokenString string) (*CustomClaims, error) {
	claims := &CustomClaims{}

	// 解析令牌
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Conf.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	// 检查令牌有效性
	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}

// StructToJSON 将结构体转换为JSON字符串
func StructToJSON(v interface{}) string {
	bytes, _ := json.Marshal(v)
	return string(bytes)
}

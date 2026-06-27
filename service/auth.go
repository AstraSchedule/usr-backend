package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrInvalidToken       = errors.New("无效的认证令牌")
)

// HashPassword 使用 bcrypt 对密码进行哈希
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(bytes), err
}

// CheckPassword 验证密码是否匹配哈希
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// JWTClaims 自定义 JWT 载荷
type JWTClaims struct {
	UserID    uint   `json:"user_id"`
	Namespace string `json:"namespace"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Scope     string `json:"scope"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT 令牌
func GenerateToken(secret string, userID uint, namespace, username, role, scope string, expiresHours int) (string, error) {
	if expiresHours <= 0 {
		expiresHours = 24
	}
	claims := JWTClaims{
		UserID:    userID,
		Namespace: namespace,
		Username:  username,
		Role:      role,
		Scope:     scope,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiresHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken 解析并验证 JWT 令牌
func ParseToken(secret, tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

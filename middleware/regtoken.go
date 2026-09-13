package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"AstraScheduleServerGo/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// RegClaims 是注册令牌携带的注册信息。
//
// 口令字段有两个：
//   - EncPassword：注册站用共享密钥加密后的口令，当前版本只写这个；
//   - Password：旧版令牌的明文口令，仅为平滑升级保留读取能力。
//
// 请使用 ResolvePassword 取明文，不要直接读取字段。
type RegClaims struct {
	Subdomain   string `json:"subdomain"`
	Username    string `json:"username"`
	EncPassword string `json:"enc_password"`
	Password    string `json:"password"`
	School      string `json:"school"`
	Grade       string `json:"grade"`
	Class       string `json:"class"`
	jwt.RegisteredClaims
}

// ResolvePassword 还原管理员明文口令。
//
// 优先解密密文字段；仅在密文缺失时回退到旧版令牌的明文字段，
// 便于注册站与后端分两次发布而不中断注册。
func (c *RegClaims) ResolvePassword(secret string) (string, error) {
	if c.EncPassword != "" {
		plaintext, err := DecryptPassword(secret, c.EncPassword)
		if err != nil {
			return "", fmt.Errorf("解密注册口令失败: %w", err)
		}
		// 空口令必须拒绝：bcrypt 接受空密码，会静默创建出空口令管理员，
		// 而不是让注册失败。旧版明文字段同样有此校验。
		if plaintext == "" {
			return "", errors.New("注册令牌未携带口令信息")
		}
		return plaintext, nil
	}

	if c.Password == "" {
		return "", errors.New("注册令牌未携带口令信息")
	}
	return c.Password, nil
}

// errInvalidRegToken 是注册令牌校验失败的统一提示。
//
// 对外一律返回同一句话，既不泄露 JWT 解析细节，也避免同一字面量反复出现。
const errInvalidRegToken = "注册令牌无效"

// rejectRegToken 以统一文案拒绝当前请求。
func rejectRegToken(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, gin.H{"detail": errInvalidRegToken})
	c.Abort()
}

// regPasswordKey 是解密后的注册口令在 gin Context 中的键。
const regPasswordKey = "reg_password"

// RegTokenAuth 验证注册令牌（X-Reg-Token header）
func RegTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("X-Reg-Token")
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "缺少注册令牌"})
			c.Abort()
			return
		}

		secret := model.Configs.Internal.Secret
		if len(secret) < MinSecretLength {
			// 密钥缺失或过短时直接拒绝，而不是用一个弱密钥去验签：
			// 空密钥下的 HMAC 任何人都能伪造。
			rejectRegToken(c)
			return
		}

		token, err := jwt.ParseWithClaims(tokenStr, &RegClaims{}, func(*jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			// 必须带 exp：缺少过期时间的令牌会被永久接受并反复进入建租户流程。
			jwt.WithExpirationRequired(),
		)
		if err != nil || !token.Valid {
			rejectRegToken(c)
			return
		}

		claims, ok := token.Claims.(*RegClaims)
		if !ok {
			rejectRegToken(c)
			return
		}

		password, err := claims.ResolvePassword(secret)
		if err != nil {
			rejectRegToken(c)
			return
		}

		c.Set(regPasswordKey, password)
		c.Set("reg_claims", claims)
		c.Next()
	}
}

// GetRegPassword 返回中间件已解密的注册口令。
func GetRegPassword(c *gin.Context) (string, bool) {
	value, exists := c.Get(regPasswordKey)
	if !exists {
		return "", false
	}
	password, ok := value.(string)
	return password, ok
}

// GetRegClaims 从 Context 获取注册令牌 claims
func GetRegClaims(c *gin.Context) *RegClaims {
	claims, exists := c.Get("reg_claims")
	if !exists {
		return nil
	}
	regClaims, ok := claims.(*RegClaims)
	if !ok {
		return nil
	}
	return regClaims
}

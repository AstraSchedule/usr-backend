package middleware

import (
	"net/http"

	"AstraScheduleServerGo/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type RegClaims struct {
	Subdomain string `json:"subdomain"`
	Username  string `json:"username"`
	jwt.RegisteredClaims
}

// RegTokenAuth 验证注册令牌（X-Reg-Token header）
func RegTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("X-Reg-Token")
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "缺少注册令牌"})
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(tokenStr, &RegClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(model.Configs.Internal.Secret), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "注册令牌无效"})
			c.Abort()
			return
		}

		claims := token.Claims.(*RegClaims)
		c.Set("reg_claims", claims)
		c.Next()
	}
}

// GetRegClaims 从 Context 获取注册令牌 claims
func GetRegClaims(c *gin.Context) *RegClaims {
	claims, exists := c.Get("reg_claims")
	if !exists {
		return nil
	}
	return claims.(*RegClaims)
}

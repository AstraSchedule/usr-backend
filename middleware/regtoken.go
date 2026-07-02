package middleware

import (
	"fmt"
	"net/http"

	"AstraScheduleServerGo/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type RegClaims struct {
	Subdomain string `json:"subdomain"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	School    string `json:"school"`
	Grade     string `json:"grade"`
	Class     string `json:"class"`
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

		secret := model.Configs.Internal.Secret
		fmt.Printf("[RegTokenAuth] token length=%d, secret length=%d\n", len(tokenStr), len(secret))

		token, err := jwt.ParseWithClaims(tokenStr, &RegClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Method)
			}
			return []byte(secret), nil
		})
		if err != nil {
			fmt.Printf("[RegTokenAuth] JWT parse error: %v\n", err)
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "注册令牌无效: " + err.Error()})
			c.Abort()
			return
		}
		if !token.Valid {
			fmt.Println("[RegTokenAuth] JWT token invalid")
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "注册令牌无效"})
			c.Abort()
			return
		}

		claims := token.Claims.(*RegClaims)
		fmt.Printf("[RegTokenAuth] OK: subdomain=%s username=%s\n", claims.Subdomain, claims.Username)
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

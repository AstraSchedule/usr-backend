package middleware

import (
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/service"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const UserClaimsKey = "user_claims"

// JWTAuthMiddleware 验证 JWT 令牌，将 claims 注入 Context
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "缺少认证令牌"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "认证格式应为 Bearer <token>"})
			c.Abort()
			return
		}

		claims, err := service.ParseToken(model.Configs.Secret.Token, tokenString)
		if err != nil {
			logrus.Debugf("JWT 验证失败: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "认证令牌无效或已过期"})
			c.Abort()
			return
		}

		c.Set(UserClaimsKey, claims)
		c.Next()
	}
}

// RequireRole 要求用户具有指定角色之一
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get(UserClaimsKey)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "未认证"})
			c.Abort()
			return
		}
		jwtClaims, ok := claims.(*service.JWTClaims)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "内部错误"})
			c.Abort()
			return
		}
		for _, role := range roles {
			if jwtClaims.Role == role {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"detail": "权限不足"})
		c.Abort()
	}
}

// GetUserClaims 从 Context 获取当前用户的 JWT claims
func GetUserClaims(c *gin.Context) *service.JWTClaims {
	claims, exists := c.Get(UserClaimsKey)
	if !exists {
		return nil
	}
	jwtClaims, ok := claims.(*service.JWTClaims)
	if !ok {
		return nil
	}
	return jwtClaims
}

package middleware

import (
	"net/http"

	"AstraScheduleServerGo/model"

	"github.com/gin-gonic/gin"
)

// InternalAuth 验证内部 API 密钥，用于服务间调用
func InternalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := c.GetHeader("X-Internal-Secret")
		if secret == "" || secret != model.Configs.Internal.Secret {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "内部认证失败"})
			c.Abort()
			return
		}
		c.Next()
	}
}

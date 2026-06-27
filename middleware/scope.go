package middleware

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireScope 校验用户对指定 school/grade/class 的写权限
func RequireScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetUserClaims(c)
		if claims == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "未认证"})
			c.Abort()
			return
		}

		school := c.Param("school")
		grade := c.Param("grade")
		classNumber := c.Param("class_number")

		if !db.CheckScopePermission(&dbTable.User{Role: claims.Role, Scope: claims.Scope}, school, grade, classNumber) {
			c.JSON(http.StatusForbidden, gin.H{"detail": "无权操作该作用域"})
			c.Abort()
			return
		}

		c.Next()
	}
}

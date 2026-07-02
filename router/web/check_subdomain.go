package web

import (
	"net/http"

	"AstraScheduleServerGo/db"

	"github.com/gin-gonic/gin"
)

// CheckSubdomainInternal 内部接口：检查 namespace 是否已存在
func CheckSubdomainInternal(c *gin.Context) {
	subdomain := c.Param("subdomain")
	if subdomain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "subdomain 不能为空"})
		return
	}

	namespace := "cn/getastra/" + subdomain

	var count int64
	db.GetDB().Table("users").Where("namespace = ?", namespace).Count(&count)

	c.JSON(http.StatusOK, gin.H{
		"exists": count > 0,
	})
}

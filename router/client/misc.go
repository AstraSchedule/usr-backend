package client

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func BroadcastSyncConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  200,
		"message": "SyncConfig",
	})
}

func WebSocketPlaceholder(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"detail": "Golang 版本暂未实现 WebSocket",
	})
}

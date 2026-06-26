package web

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func ListUsers(c *gin.Context) {
	users, err := db.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(users))
	for _, u := range users {
		out = append(out, gin.H{
			"id":              u.ID,
			"username":        u.Username,
			"role":            u.Role,
			"scope":              u.Scope,
			"must_change_pwd":    u.MustChangePwd,
			"must_change_username": u.MustChangeUsername,
			"created_at":         u.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func CreateUser(c *gin.Context) {
	var req struct {
		Username          string `json:"username"`
		Password          string `json:"password"`
		Role              string `json:"role"`
		Scope             string `json:"scope"`
		MustChangePwd     *bool  `json:"must_change_pwd"`
		MustChangeUsername *bool `json:"must_change_username"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数"})
		return
	}

	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "用户名和密码不能为空"})
		return
	}

	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "密码长度不能少于 6 位"})
		return
	}

	validRoles := map[string]bool{"admin": true, "school_w": true, "grade_w": true, "class_w": true, "readonly": true}
	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的角色，可选: admin, school_w, grade_w, class_w, readonly"})
		return
	}

	hash, err := service.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "密码哈希失败"})
		return
	}

	user := dbTable.User{
		Username:           req.Username,
		PasswordHash:       hash,
		Role:               req.Role,
		Scope:              req.Scope,
		MustChangePwd:      req.MustChangePwd != nil && *req.MustChangePwd,
		MustChangeUsername: req.MustChangeUsername != nil && *req.MustChangeUsername,
	}

	if err := db.CreateUser(&user); err != nil {
		c.JSON(http.StatusConflict, gin.H{"detail": "用户名已存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": 200,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
			"scope":    user.Scope,
		},
	})
}

func UpdateUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的用户 ID"})
		return
	}

	user, err := db.GetUserByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "用户不存在"})
		return
	}

	var req struct {
		Username           *string `json:"username"`
		Password           *string `json:"password"`
		Role               *string `json:"role"`
		Scope              *string `json:"scope"`
		MustChangePwd      *bool   `json:"must_change_pwd"`
		MustChangeUsername *bool   `json:"must_change_username"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数"})
		return
	}

	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.Password != nil {
		if len(*req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "密码长度不能少于 6 位"})
			return
		}
		hash, err := service.HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "密码哈希失败"})
			return
		}
		user.PasswordHash = hash
		user.MustChangePwd = false
	}
	if req.Role != nil {
		validRoles := map[string]bool{"admin": true, "school_w": true, "grade_w": true, "class_w": true, "readonly": true}
		if !validRoles[*req.Role] {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的角色"})
			return
		}
		user.Role = *req.Role
	}
	if req.Scope != nil {
		user.Scope = *req.Scope
	}
	if req.MustChangePwd != nil {
		user.MustChangePwd = *req.MustChangePwd
	}
	if req.MustChangeUsername != nil {
		user.MustChangeUsername = *req.MustChangeUsername
	}

	if err := db.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": 200,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
			"scope":    user.Scope,
		},
	})
}

func DeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的用户 ID"})
		return
	}

	affected, err := db.DeleteUser(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "用户不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": 200, "deleted": affected})
}

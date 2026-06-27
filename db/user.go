package db

import (
	"AstraScheduleServerGo/model/dbTable"

	"gorm.io/gorm/clause"
)

func GetUserByUsername(namespace, username string) (*dbTable.User, error) {
	user := &dbTable.User{}
	err := GetDB().Where("namespace = ? AND username = ?", namespace, username).Take(user).Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func GetUserByID(id uint) (*dbTable.User, error) {
	user := &dbTable.User{}
	err := GetDB().Where("id = ?", id).Take(user).Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func ListUsers(namespace string) ([]dbTable.User, error) {
	users := make([]dbTable.User, 0)
	q := GetDB().Order("id ASC")
	if namespace != "" {
		q = q.Where("namespace = ?", namespace)
	}
	err := q.Find(&users).Error
	return users, err
}

func CreateUser(user *dbTable.User) error {
	return GetDB().Create(user).Error
}

func UpdateUser(user *dbTable.User) error {
	return GetDB().Save(user).Error
}

func DeleteUser(id uint) (int64, error) {
	resp := GetDB().Where("id = ?", id).Delete(&dbTable.User{})
	return resp.RowsAffected, resp.Error
}

// UpsertUser 按 namespace+username upsert（启动时创建默认管理员用）
func UpsertUser(user *dbTable.User) error {
	return GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "namespace"}, {Name: "username"}},
		DoNothing: true,
	}).Create(user).Error
}

// CountUsers 返回指定 namespace 的用户总数
func CountUsers(namespace string) (int64, error) {
	var count int64
	q := GetDB().Model(&dbTable.User{})
	if namespace != "" {
		q = q.Where("namespace = ?", namespace)
	}
	err := q.Count(&count).Error
	return count, err
}

// UpdatePassword 更新用户密码哈希并清除 must_change_pwd 标志
func UpdatePassword(userID uint, hash string) error {
	return GetDB().Model(&dbTable.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{
			"password_hash":   hash,
			"must_change_pwd": false,
		}).Error
}

// EnsureAdminUser 确保至少存在一个管理员账户，若无用户则创建 admin/admin
func EnsureAdminUser(namespace string) {
	count, err := CountUsers(namespace)
	if err != nil {
		return
	}
	if count > 0 {
		return
	}
	// 由调用方在 startup 中完成创建逻辑
}

// UserScopeContains 检查用户权限是否覆盖指定 scope
func UserScopeContains(user *dbTable.User, targetScope string) bool {
	if user.Role == "admin" {
		return true
	}
	if user.Scope == "" {
		return false
	}
	if user.Scope == targetScope {
		return true
	}
	if len(user.Scope) < len(targetScope) {
		prefix := targetScope[:len(user.Scope)]
		if prefix == user.Scope && targetScope[len(user.Scope)] == '/' {
			return true
		}
	}
	return false
}

// CheckScopePermission 检查用户对指定 school/grade/class 的读写权限
func CheckScopePermission(user *dbTable.User, school, grade, class string) bool {
	if user.Role == "admin" {
		return true
	}
	switch user.Role {
	case "school_w":
		return user.Scope == school
	case "grade_w":
		return user.Scope == school+"/"+grade
	case "class_w":
		return user.Scope == school+"/"+grade+"/"+class
	default:
		return false
	}
}

// CheckGradePermission 检查用户对指定 school/grade 的读写权限
func CheckGradePermission(user *dbTable.User, school, grade string) bool {
	if user.Role == "admin" {
		return true
	}
	switch user.Role {
	case "school_w":
		return user.Scope == school
	case "grade_w":
		return user.Scope == school+"/"+grade
	case "class_w":
		return false
	default:
		return false
	}
}

// CheckSchoolPermission 检查用户对指定 school 的读写权限
func CheckSchoolPermission(user *dbTable.User, school string) bool {
	if user.Role == "admin" {
		return true
	}
	if user.Role == "school_w" {
		return user.Scope == school
	}
	return false
}

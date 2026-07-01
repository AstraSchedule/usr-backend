package startup

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/service"

	"github.com/sirupsen/logrus"
)

func StartInit() {
	ReadConfig()
	SetLog()
	MigrateDb()
	EnsureAdminUser()
}

func EnsureAdminUser() {
	count, err := db.CountUsers()
	if err != nil {
		logrus.Warnf("检查用户数量失败: %v", err)
		return
	}
	if count > 0 {
		return
	}

	hash, err := service.HashPassword("admin")
	if err != nil {
		logrus.Errorf("生成默认管理员密码哈希失败: %v", err)
		return
	}

	admin := &dbTable.User{
		Username:      "admin",
		PasswordHash:  hash,
		Role:          "admin",
		Scope:         "",
		MustChangePwd: true,
	}

	if err := db.CreateUser(admin); err != nil {
		logrus.Errorf("创建默认管理员账户失败: %v", err)
		return
	}

	logrus.Infof("已创建默认管理员账户: admin/admin（首次登录需修改密码）")
}

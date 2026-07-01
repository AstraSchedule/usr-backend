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
	// SaaS 模式不自动创建 default namespace，用户通过 Dashboard 管理
}

func EnsureAdminUser(namespace string) {
	count, err := db.CountUsers(namespace)
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
		Namespace:     namespace,
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

	logrus.Infof("已创建默认管理员账户: admin/admin（命名空间: %s，首次登录需修改密码）", namespace)
}

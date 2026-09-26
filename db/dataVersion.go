package db

import (
	"AstraScheduleServerGo/model/dbTable"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BumpDataVersion 用给定连接把作用域 (school, grade, class) 的数据版本推进到 at。
// 连接由调用方传入：事务内必须传 tx —— 测试用的 SQLite 是单连接（SetMaxOpenConns(1)），
// 事务占着连接时再用 GetDB() 会等待自己，直接死锁。
// 用于「删除」这类不会自然产生更新时间戳的操作：记录连同它的 UpdatedAt 一起消失，
// 若不显式推进版本，客户端会一直命中 304（边缘缓存同理），继续展示已删除的配置。
// 传空串表示「全局」：删除的作用域可能粗于单个班级（例如年级级自动任务），
// 此时用一条全局版本行兜底，代价是同一命名空间内所有班级各回源一次——删除属低频操作。
func BumpDataVersion(conn *gorm.DB, namespace, school, grade, class string, at time.Time) error {
	// 版本必须严格递增：LatestTimestamp 会截断到 Unix 秒，同一秒内的两次删除若拿到相同秒值，
	// 客户端会继续命中 304、沿用已删除的配置。因此与现有版本比较后至少 +1 秒。
	next := dateOnlySecond(at)
	existing := dbTable.DataVersion{}
	conn.Where("namespace = ? AND school = ? AND grade = ? AND class = ?", namespace, school, grade, class).Take(&existing)
	if !existing.Version.IsZero() {
		if bumped := existing.Version.Add(time.Second); !next.After(bumped) {
			next = bumped
		}
	}
	row := dbTable.DataVersion{Namespace: namespace, School: school, Grade: grade, Class: class, Version: next}
	return conn.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "namespace"}, {Name: "school"}, {Name: "grade"}, {Name: "class"}},
		DoUpdates: clause.AssignmentColumns([]string{"version", "updated_at"}),
	}).Create(&row).Error
}

// dateOnlySecond 截断到秒：版本比较走 LatestTimestamp（Unix 秒），纳秒不参与比较
func dateOnlySecond(t time.Time) time.Time {
	return time.Unix(t.Unix(), 0)
}

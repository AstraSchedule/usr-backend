package db

import (
	"AstraScheduleServerGo/model/dbTable"
	"time"

	"gorm.io/gorm/clause"
)

// BumpDataVersion 把作用域 (school, grade, class) 的数据版本推进到 at。
// 用于「删除」这类不会自然产生更新时间戳的操作：记录连同它的 UpdatedAt 一起消失，
// 若不显式推进版本，客户端会一直命中 304（边缘缓存同理），继续展示已删除的配置。
// 传空串表示「全局」：删除的作用域可能粗于单个班级（例如年级级自动任务），
// 此时用一条全局版本行兜底，代价是同一命名空间内所有班级各回源一次——删除属低频操作。
func BumpDataVersion(school, grade, class string, at time.Time) error {
	row := dbTable.DataVersion{School: school, Grade: grade, Class: class, Version: at}
	return GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "school"}, {Name: "grade"}, {Name: "class"}},
		DoUpdates: clause.AssignmentColumns([]string{"version", "updated_at"}),
	}).Create(&row).Error
}

package service

import (
	"time"

	"AstraScheduleServerGo/model/dbTable"
)

// LatestApplicableRecordTimestamp 返回命中该班级作用域的自动任务记录中最新的更新时间。
//
// 自动任务不改写课表/作息等数据行，只改变读取时的响应结果，所以它的版本贡献只能来自
// 记录自身的时间戳：规则的新增、编辑、停用都必须让客户端重新拉取。
// 与 VersionBoundary 互补——后者负责"时间到点后的命中翻转"，这里负责"规则本身被改动"。
func LatestApplicableRecordTimestamp(records []dbTable.AutorunRecord, school, grade, classNumber string) time.Time {
	latest := time.Time{}
	for _, record := range records {
		if bestRowSpecificity(record.Scope, school, grade, classNumber) < 0 {
			continue
		}
		if record.UpdatedAt.After(latest) {
			latest = record.UpdatedAt
		}
	}
	return latest
}

// LatestCountdownTimestamp 返回给定倒数日记录中最新的更新时间（调用方已按作用域过滤）
func LatestCountdownTimestamp(records []dbTable.CountdownRecord) time.Time {
	latest := time.Time{}
	for _, record := range records {
		if record.UpdatedAt.After(latest) {
			latest = record.UpdatedAt
		}
	}
	return latest
}

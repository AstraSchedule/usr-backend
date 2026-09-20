package service

import (
	"testing"
	"time"

	"AstraScheduleServerGo/model/dbTable"

	"github.com/stretchr/testify/assert"
)

// 自动任务不写任何数据行，只改变读取时的响应结果，因此版本必须能反映"规则被改动"。
func TestLatestApplicableRecordTimestamp_RespectsScope(t *testing.T) {
	base := time.Unix(1700000000, 0)
	classLevel := base.Add(2 * time.Hour)
	gradeLevel := base.Add(1 * time.Hour)
	otherClass := base.Add(5 * time.Hour)

	records := []dbTable.AutorunRecord{
		{Scope: []string{"other/2024/1"}, UpdatedAt: otherClass},
		{Scope: []string{"school/2024"}, UpdatedAt: gradeLevel},
		{Scope: []string{"school/2024/7"}, UpdatedAt: classLevel},
	}

	assert.Equal(t, classLevel, LatestApplicableRecordTimestamp(records, "school", "2024", "7"))
	// 同名但不同班级的记录不能影响本班级
	assert.Equal(t, gradeLevel, LatestApplicableRecordTimestamp(records, "school", "2024", "8"))
	assert.True(t, LatestApplicableRecordTimestamp(nil, "school", "2024", "7").IsZero())
}

func TestLatestCountdownTimestamp(t *testing.T) {
	older := time.Unix(1700000000, 0)
	newer := time.Unix(1750000000, 0)

	records := []dbTable.CountdownRecord{{UpdatedAt: older}, {UpdatedAt: newer}}
	assert.Equal(t, newer, LatestCountdownTimestamp(records))
	assert.True(t, LatestCountdownTimestamp(nil).IsZero())
}

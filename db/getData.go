package db

import (
	"time"

	"AstraScheduleServerGo/model/dbTable"

	"github.com/dromara/carbon/v2"
)

const (
	scopeClassWhere = "school = ? AND grade = ? AND class = ?"
	scopeGradeWhere = "school = ? AND grade = ?"
)

// LatestTimestamp 取一组时间戳中最新的一个（Unix 秒）；全部为零值时间时返回 0。
//
// 绝不能直接把零值 time.Time 的 Unix() 下发：它是 -62135596800（0001-01-01），
// 客户端会把它当成正常版本回传，而服务端比较又恒等，于是 304 会把旧配置一直钉住。
func LatestTimestamp(times ...time.Time) int64 {
	latest := time.Time{}
	for _, t := range times {
		if t.After(latest) {
			latest = t
		}
	}
	if latest.IsZero() {
		return 0
	}
	return latest.Unix()
}

// GetDataVersion 返回显式版本行；不存在时返回零值行（由调用方结合其它时间戳兜底）
func GetDataVersion(school, grade, class string) dbTable.DataVersion {
	dataVersion := dbTable.DataVersion{}
	GetDB().Where(scopeClassWhere, school, grade, class).Take(&dataVersion)
	return dataVersion
}

// GetLatestVersion 仅返回显式版本行的时间戳（缺失或零值时返回 0）
func GetLatestVersion(school, grade, class string) *carbon.Carbon {
	return carbon.CreateFromTimestamp(LatestTimestamp(GetDataVersion(school, grade, class).Version))
}

func GetClientConfig(school, grade, class string) *dbTable.ClientConfig {
	clientConfig := &dbTable.ClientConfig{}
	GetDB().Where(scopeClassWhere, school, grade, class).Take(clientConfig)
	return clientConfig
}

func GetSchedule(school, grade, class string) *dbTable.Schedule {
	schedule := &dbTable.Schedule{}
	GetDB().Where(scopeClassWhere, school, grade, class).Take(schedule)
	return schedule
}

func GetSubject(school, grade string) *dbTable.Subject {
	subject := &dbTable.Subject{}
	GetDB().Where(scopeGradeWhere, school, grade).Take(subject)
	return subject
}

func GetTimetable(school, grade string) *dbTable.Timetable {
	timetable := &dbTable.Timetable{}
	GetDB().Where(scopeGradeWhere, school, grade).Take(timetable)
	return timetable
}

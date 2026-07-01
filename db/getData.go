package db

import (
	"AstraScheduleServerGo/model/dbTable"

	"github.com/dromara/carbon/v2"
)

const (
	scopeClassWhere = "school = ? AND grade = ? AND class = ?"
	scopeGradeWhere = "school = ? AND grade = ?"
)

func GetLatestVersion(school, grade, class string) *carbon.Carbon {
	latestVersion := &dbTable.DataVersion{}
	GetDB().Where(scopeClassWhere, school, grade, class).Take(latestVersion)
	return carbon.CreateFromTimestamp(latestVersion.Version.Unix())
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

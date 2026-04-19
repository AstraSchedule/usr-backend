package db

import (
	"AstraScheduleServerGo/model/dbTable"

	"github.com/dromara/carbon/v2"
)

const (
	scopeClassWhere = "school = ? AND grade = ? AND class = ?"
	scopeGradeWhere = "school = ? AND grade = ?"
)

func GetLatestVersion(school string, grade string, class string) *carbon.Carbon {
	latestVersion := &dbTable.DataVersion{}
	GetDB().Where(scopeClassWhere, school, grade, class).Take(latestVersion)
	return carbon.CreateFromTimestamp(latestVersion.Version.Unix())
}

func GetClientConfig(school string, grade string, class string) *dbTable.ClientConfig {
	clientConfig := &dbTable.ClientConfig{}
	GetDB().Where(scopeClassWhere, school, grade, class).Take(clientConfig)
	return clientConfig
}

func GetSchedule(school string, grade string, class string) *dbTable.Schedule {
	schedule := &dbTable.Schedule{}
	GetDB().Where(scopeClassWhere, school, grade, class).Take(schedule)
	return schedule
}

func GetSubject(school string, grade string) *dbTable.Subject {
	subject := &dbTable.Subject{}
	GetDB().Where(scopeGradeWhere, school, grade).Take(subject)
	return subject
}

func GetTimetable(school string, grade string) *dbTable.Timetable {
	timetable := &dbTable.Timetable{}
	GetDB().Where(scopeGradeWhere, school, grade).Take(timetable)
	return timetable
}

package db

import (
	"AstraScheduleServerGo/model"
	"AstraScheduleServerGo/model/dbTable"

	"github.com/dromara/carbon/v2"
)

func GetLatestVersion(school string, grade string, class string) *carbon.Carbon {
	latestVersion := &dbTable.DataVersion{}
	model.Db.Where("school = ? AND grade = ? AND class = ?", school, grade, class).Take(latestVersion)
	return carbon.CreateFromTimestamp(latestVersion.Version.Unix())
}

func GetClientConfig(school string, grade string, class string) *dbTable.ClientConfig {
	clientConfig := &dbTable.ClientConfig{}
	model.Db.Where("school = ? AND grade = ? AND class = ?", school, grade, class).Take(clientConfig)
	return clientConfig
}

func GetSchedule(school string, grade string, class string) *dbTable.Schedule {
	schedule := &dbTable.Schedule{}
	model.Db.Where("school = ? AND grade = ? AND class = ?", school, grade, class).Take(schedule)
	return schedule
}

func GetSubject(school string, grade string) *dbTable.Subject {
	subject := &dbTable.Subject{}
	model.Db.Where("school = ? AND grade = ?", school, grade).Take(subject)
	return subject
}

func GetTimetable(school string, grade string) *dbTable.Timetable {
	timetable := &dbTable.Timetable{}
	model.Db.Where("school = ? AND grade = ?", school, grade).Take(timetable)
	return timetable
}

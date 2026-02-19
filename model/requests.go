package model

import "AstraScheduleServerGo/model/dbTable"

type FullClientConfig struct {
	SupportWebsocket bool                  `json:"supportWebSocket"`
	DailyClasses     [7]dbTable.DailyClass `json:"daily_class"`
	dbTable.ClientConfigItems
	dbTable.TimetableConfig
	dbTable.SubjectConfig
}

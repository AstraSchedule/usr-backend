package model

import "AstraScheduleServerGo/model/dbTable"

type FullClientConfig struct {
	DailyClasses [7]dbTable.DailyClass `json:"daily_class"`
	dbTable.ClientConfigItems
	dbTable.TimetableConfig
	dbTable.SubjectConfig
}

type FullResponseConfig struct {
	SupportWebsocket bool                  `json:"supportWebSocket"`
	Version          string                `json:"version"`
	DailyClasses     [7]dbTable.DailyClass `json:"daily_class"`
	dbTable.ClientConfigItems
	dbTable.TimetableConfig
	dbTable.SubjectConfig
	CountdownRecords []dbTable.CountdownRecord `json:"countdown_records,omitempty"`
}

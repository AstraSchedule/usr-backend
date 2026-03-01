package web

type textItem struct {
	Text string `json:"text"`
}

type subjectsPayload struct {
	Abbr     []textItem `json:"abbr"`
	FullName []textItem `json:"fullName"`
}

type dailyClassInput struct {
	Chinese   string     `json:"Chinese"`
	English   string     `json:"English"`
	ClassList [][]string `json:"classList"`
	Timetable string     `json:"timetable"`
}

type schedulePayload struct {
	DailyClass []dailyClassInput `json:"daily_class"`
}

type autorunPayload struct {
	Type     int                    `json:"type"`
	Scope    interface{}            `json:"scope"`
	Priority int                    `json:"priority"`
	ID       string                 `json:"id"`
	Content  map[string]interface{} `json:"content"`
}

type copyScopePayload struct {
	School      string `json:"school"`
	Grade       string `json:"grade"`
	Class       string `json:"class"`
	ClassNumber string `json:"class_number"`
}

func (s copyScopePayload) ClassValue() string {
	if s.Class != "" {
		return s.Class
	}
	return s.ClassNumber
}

type copyConfigPayload struct {
	From copyScopePayload `json:"from"`
	To   copyScopePayload `json:"to"`
}

package service

import (
	"AstraScheduleServerGo/model/dbTable"
	"strings"
)

// ScopeMatchesClass 检查 scope 是否匹配指定的 classID（格式：school/grade/class）
func ScopeMatchesClass(scope, classID string) bool {
	if scope == "" || scope == "ALL" {
		return true
	}
	sParts := strings.Split(scope, "/")
	cParts := strings.Split(classID, "/")
	if len(cParts) < 3 {
		return false
	}
	school, grade, classNumber := cParts[0], cParts[1], cParts[2]
	switch len(sParts) {
	case 1:
		return sParts[0] == school
	case 2:
		return sParts[0] == school && sParts[1] == grade
	default:
		return sParts[0] == school && sParts[1] == grade && sParts[2] == classNumber
	}
}

// FilterCountdownByScope 按 classID 过滤倒数日记录
func FilterCountdownByScope(records []dbTable.CountdownRecord, classID string) []dbTable.CountdownRecord {
	if classID == "" {
		return records
	}
	out := make([]dbTable.CountdownRecord, 0, len(records))
	for _, rec := range records {
		scopes := rec.Scope
		if len(scopes) == 0 {
			scopes = []string{"ALL"}
		}
		matched := false
		for _, scope := range scopes {
			if ScopeMatchesClass(scope, classID) {
				matched = true
				break
			}
		}
		if matched {
			out = append(out, rec)
		}
	}
	return out
}

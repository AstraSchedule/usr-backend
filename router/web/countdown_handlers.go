package web

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func scopeMatchesClass(scope, classID string) bool {
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

func filterCountdownByScope(records []dbTable.CountdownRecord, classID string) []dbTable.CountdownRecord {
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
			if scopeMatchesClass(scope, classID) {
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

func normalizeCountdownSchedules(items []countdownScheduleInput) []dbTable.CountdownScheduleItem {
	out := make([]dbTable.CountdownScheduleItem, 0, len(items))
	for _, it := range items {
		name := strings.TrimSpace(it.Name)
		date := strings.TrimSpace(it.Date)
		if name == "" || date == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", date); err != nil {
			continue
		}
		out = append(out, dbTable.CountdownScheduleItem{
			Name:     name,
			Date:     date,
			Priority: it.Priority,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].Date < out[j].Date
	})
	return out
}

func makeCountdownID(scope []string, schedules []dbTable.CountdownScheduleItem) string {
	parts := append([]string(nil), scope...)
	sort.Strings(parts)
	buf := strings.Join(parts, ";") + "|"
	for _, s := range schedules {
		buf += s.Name + "," + s.Date + "," + strconv.Itoa(s.Priority) + ";"
	}
	sum := sha256.Sum256([]byte(buf))
	return hex.EncodeToString(sum[:])[:16]
}

func mapCountdownRecord(r dbTable.CountdownRecord) gin.H {
	return gin.H{
		"id":        r.ID,
		"scope":     r.Scope,
		"schedules": r.Schedules,
	}
}

func GetCountdownStatus(c *gin.Context) {
	scope := strings.TrimSpace(c.Query("scope"))
	rows, err := db.FetchCountdownRecords("")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(rows) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"loading":   false,
			"hasConfig": false,
			"data":      []gin.H{},
		})
		return
	}

	rows = filterCountdownByScope(rows, scope)
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, mapCountdownRecord(r))
	}

	c.JSON(http.StatusOK, gin.H{
		"loading":   false,
		"hasConfig": true,
		"data":      out,
	})
}

func GetCountdownByID(c *gin.Context) {
	id := c.Param("id")
	rows, err := db.FetchCountdownRecords(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(rows) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": mapCountdownRecord(rows[0])})
}

func PutCountdownRule(c *gin.Context) {
	var payload countdownPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数: " + err.Error()})
		return
	}
	scope := parseScopeInput(payload.Scope)
	schedules := normalizeCountdownSchedules(payload.Schedules)
	if len(schedules) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "schedules 不能为空，且每项需要合法 name/date(YYYY-MM-DD)"})
		return
	}

	recordID := strings.TrimSpace(payload.ID)
	if recordID == "" {
		recordID = makeCountdownID(scope, schedules)
	}
	record := dbTable.CountdownRecord{
		ID:        recordID,
		Scope:     scope,
		Schedules: schedules,
	}
	if err := db.UpsertCountdownRecord(&record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": 200, "id": recordID})
}

func DeleteCountdownRecord(c *gin.Context) {
	id := c.Param("id")
	affected, err := db.DeleteCountdownRecord(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "记录不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": 200, "deleted": affected, "id": id})
}

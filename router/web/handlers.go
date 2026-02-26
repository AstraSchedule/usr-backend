package web

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/service"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/daizihan233/go-valence-cal"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

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

func parseScopeInput(raw interface{}) []string {
	if raw == nil {
		return []string{"ALL"}
	}
	switch v := raw.(type) {
	case string:
		if v == "" {
			return []string{"ALL"}
		}
		return []string{v}
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if ok && s != "" {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return []string{"ALL"}
		}
		return out
	case []string:
		if len(v) == 0 {
			return []string{"ALL"}
		}
		return v
	default:
		return []string{"ALL"}
	}
}

func makeHashID(etype int, scope []string, level int, parameters map[string]interface{}) string {
	sum := sha256.Sum256([]byte(strconv.Itoa(etype) + "|" + strconv.Itoa(level) + "|" + stringsFromScope(scope) + "|" + stableMapString(parameters)))
	return hex.EncodeToString(sum[:])[:16]
}

func stringsFromScope(scope []string) string {
	copyScope := append([]string(nil), scope...)
	sort.Strings(copyScope)
	out := ""
	for _, s := range copyScope {
		out += s + ";"
	}
	return out
}

func stableMapString(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := ""
	for _, k := range keys {
		out += k + "=" + toString(m[k]) + "|"
	}
	return out
}

func toString(v interface{}) string {
	switch vv := v.(type) {
	case string:
		return vv
	case float64:
		return strconv.FormatFloat(vv, 'f', -1, 64)
	case int:
		return strconv.Itoa(vv)
	case bool:
		if vv {
			return "true"
		}
		return "false"
	case map[string]interface{}:
		return stableMapString(vv)
	case []interface{}:
		out := ""
		for _, x := range vv {
			out += toString(x) + ","
		}
		return out
	default:
		return ""
	}
}

func mapAutorunRecord(r dbTable.AutorunRecord) gin.H {
	return gin.H{
		"id":       r.HashID,
		"hashid":   r.HashID,
		"type":     r.EType,
		"priority": r.Level,
		"status":   r.Status,
		"scope":    r.Scope,
		"content":  r.Parameters,
	}
}

func parseClassList(input [][]string) []string {
	if len(input) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(input))
	for _, item := range input {
		if len(item) == 0 {
			out = append(out, "")
			continue
		}
		out = append(out, item[0])
	}
	return out
}

func GetStatistic(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"weather_error":              0,
		"websocket_disconnect":       gin.H{},
		"websocket_disconnect_count": 0,
		"clients":                    []string{},
		"clients_count":              0,
	})
}

func GetSubjectsOptions(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	subject := db.GetSubject(school, grade)
	options := make([]gin.H, 0)
	for abbr, full := range subject.SubjectName {
		options = append(options, gin.H{"label": abbr + "（" + full + "）", "value": abbr})
	}
	c.JSON(http.StatusOK, gin.H{"options": options})
}

func GetSubjects(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	subject := db.GetSubject(school, grade)
	abbr := make([]gin.H, 0)
	fullName := make([]gin.H, 0)
	for k, v := range subject.SubjectName {
		abbr = append(abbr, gin.H{"text": k})
		fullName = append(fullName, gin.H{"text": v})
	}
	c.JSON(http.StatusOK, gin.H{"abbr": abbr, "fullName": fullName})
}

func PutSubjects(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	bodyMap := raw
	if modelVal, ok := raw["model"].(map[string]interface{}); ok {
		bodyMap = modelVal
	}
	body := subjectsPayload{}
	if arr, ok := bodyMap["abbr"].([]interface{}); ok {
		for _, item := range arr {
			if obj, ok := item.(map[string]interface{}); ok {
				if text, ok := obj["text"].(string); ok {
					body.Abbr = append(body.Abbr, textItem{Text: text})
				}
			}
		}
	}
	if arr, ok := bodyMap["fullName"].([]interface{}); ok {
		for _, item := range arr {
			if obj, ok := item.(map[string]interface{}); ok {
				if text, ok := obj["text"].(string); ok {
					body.FullName = append(body.FullName, textItem{Text: text})
				}
			}
		}
	}
	m := map[string]string{}
	limit := len(body.Abbr)
	if len(body.FullName) < limit {
		limit = len(body.FullName)
	}
	for i := 0; i < limit; i++ {
		m[body.Abbr[i].Text] = body.FullName[i].Text
	}
	record := dbTable.Subject{School: school, Grade: grade, SubjectConfig: dbTable.SubjectConfig{SubjectName: m}}
	if err := db.GetDB().Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "school"}, {Name: "grade"}}, UpdateAll: true}).Create(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": 200})
}

func GetTimetableOptions(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	timetable := db.GetTimetable(school, grade)
	options := make([]gin.H, 0)
	for name, config := range timetable.Timetable {
		need := 0
		for _, v := range config {
			i, ok := serviceAsInt(v)
			if !ok {
				continue
			}
			if i > need {
				need = i
			}
		}
		options = append(options, gin.H{"label": name, "value": name, "need": need})
	}
	c.JSON(http.StatusOK, gin.H{"options": options})
}

func serviceAsInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		i, err := strconv.Atoi(n)
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

func GetTimetable(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	timetable := db.GetTimetable(school, grade)
	c.JSON(http.StatusOK, timetable.TimetableConfig)
}

func PutTimetable(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	var body dbTable.TimetableConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	record := dbTable.Timetable{School: school, Grade: grade, TimetableConfig: body}
	if err := db.GetDB().Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "school"}, {Name: "grade"}}, UpdateAll: true}).Create(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": 200})
}

func GetScheduleConfig(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	classNumber := c.Param("class_number")
	schedule := db.GetSchedule(school, grade, classNumber)
	timetable := db.GetTimetable(school, grade)
	maxSubjects := 0
	for _, v := range timetable.Timetable {
		for _, item := range v {
			i, ok := serviceAsInt(item)
			if ok && i+1 > maxSubjects {
				maxSubjects = i + 1
			}
		}
	}
	out := make([]gin.H, 0, 7)
	for i := 0; i < 7; i++ {
		day := schedule.DailyClasses[i]
		if _, ok := timetable.Timetable[day.Timetable]; !ok {
			if _, exists := timetable.Timetable["常日"]; exists {
				day.Timetable = "常日"
			}
		}
		classList := make([][]string, 0, len(day.ClassList))
		for _, s := range day.ClassList {
			classList = append(classList, []string{s})
		}
		for len(classList) < maxSubjects {
			classList = append(classList, []string{"课"})
		}
		out = append(out, gin.H{
			"Chinese":   day.Chinese,
			"English":   day.English,
			"classList": classList,
			"timetable": day.Timetable,
		})
	}
	c.JSON(http.StatusOK, gin.H{"daily_class": out})
}

func PutScheduleConfig(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	classNumber := c.Param("class_number")

	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	bodyMap := raw
	if modelVal, ok := raw["model"].(map[string]interface{}); ok {
		bodyMap = modelVal
	}
	body := schedulePayload{}
	if arr, ok := bodyMap["daily_class"].([]interface{}); ok {
		for _, one := range arr {
			obj, ok := one.(map[string]interface{})
			if !ok {
				continue
			}
			item := dailyClassInput{}
			item.Chinese, _ = obj["Chinese"].(string)
			item.English, _ = obj["English"].(string)
			item.Timetable, _ = obj["timetable"].(string)
			if classListRaw, ok := obj["classList"].([]interface{}); ok {
				for _, classItem := range classListRaw {
					if arr2, ok := classItem.([]interface{}); ok {
						line := make([]string, 0, len(arr2))
						for _, x := range arr2 {
							if s, ok := x.(string); ok {
								line = append(line, s)
							}
						}
						item.ClassList = append(item.ClassList, line)
					}
				}
			}
			body.DailyClass = append(body.DailyClass, item)
		}
	}

	var daily [7]dbTable.DailyClass
	for i := 0; i < 7 && i < len(body.DailyClass); i++ {
		daily[i] = dbTable.DailyClass{
			Chinese:   body.DailyClass[i].Chinese,
			English:   body.DailyClass[i].English,
			ClassList: parseClassList(body.DailyClass[i].ClassList),
			Timetable: body.DailyClass[i].Timetable,
		}
	}
	timetable := db.GetTimetable(school, grade)
	service.FixWrongTimetable(&daily, timetable.Timetable)

	record := dbTable.Schedule{School: school, Grade: grade, Class: classNumber, DailyClasses: daily}
	if err := db.GetDB().Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "school"}, {Name: "grade"}, {Name: "class"}}, UpdateAll: true}).Create(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": 200})
}

func GetSettings(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	classNumber := c.Param("class_number")
	config := db.GetClientConfig(school, grade, classNumber)
	c.JSON(http.StatusOK, config.ClientConfigItems)
}

func PutSettings(c *gin.Context) {
	school := c.Param("school")
	grade := c.Param("grade")
	classNumber := c.Param("class_number")
	var body dbTable.ClientConfigItems
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	record := dbTable.ClientConfig{School: school, Grade: grade, Class: classNumber, ClientConfigItems: body}
	if err := db.GetDB().Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "school"}, {Name: "grade"}, {Name: "class"}}, UpdateAll: true}).Create(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": 200})
}

func listSchools() ([]string, error) {
	type row struct{ School string }
	rows := make([]row, 0)
	err := db.GetDB().Model(&dbTable.Schedule{}).Distinct("school").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.School)
	}
	sort.Strings(out)
	return out, nil
}

func listGrades(school string) ([]string, error) {
	type row struct{ Grade string }
	rows := make([]row, 0)
	err := db.GetDB().Model(&dbTable.Schedule{}).Where("school = ?", school).Distinct("grade").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Grade)
	}
	sort.Strings(out)
	return out, nil
}

func listClasses(school, grade string) ([]string, error) {
	type row struct{ Class string }
	rows := make([]row, 0)
	err := db.GetDB().Model(&dbTable.Schedule{}).Where("school = ? AND grade = ?", school, grade).Distinct("class").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Class)
	}
	sort.Strings(out)
	return out, nil
}

func GetMenu(c *gin.Context) {
	menu := gin.H{"data": []gin.H{{"to": "/", "text": "总览", "key": "go-back-home", "children": nil}, {"to": "/autorun", "text": "自动任务", "key": "autorun", "children": nil}}}
	data := menu["data"].([]gin.H)
	schools, err := listSchools()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, school := range schools {
		grades, _ := listGrades(school)
		gradeChildren := make([]gin.H, 0)
		for _, grade := range grades {
			classes, _ := listClasses(school, grade)
			children := []gin.H{
				{"to": "/config/" + school + "/" + grade + "/subjects", "text": "课程设置", "key": "school-" + school + "-grade-" + grade + "-subjects", "children": nil},
				{"to": "/config/" + school + "/" + grade + "/timetable", "text": "作息设置", "key": "school-" + school + "-grade-" + grade + "-timetable", "children": nil},
			}
			for _, classNumber := range classes {
				children = append(children, gin.H{
					"text": classNumber + " 班",
					"key":  "school-" + school + "-grade-" + grade + "-class-" + classNumber,
					"raw":  classNumber,
					"children": []gin.H{
						{"to": "/config/" + school + "/" + grade + "/" + classNumber + "/schedule", "text": "课表设置", "key": "school-" + school + "-grade-" + grade + "-class-" + classNumber + "-schedule", "children": nil},
						{"to": "/config/" + school + "/" + grade + "/" + classNumber + "/settings", "text": "通用设置", "key": "school-" + school + "-grade-" + grade + "-class-" + classNumber + "-settings", "children": nil},
					},
				})
			}
			gradeChildren = append(gradeChildren, gin.H{"text": grade + " 级", "key": "school-" + school + "-grade-" + grade, "raw": grade, "children": children})
		}
		data = append(data, gin.H{"text": school + " 学校", "key": "school-" + school, "raw": school, "children": gradeChildren})
	}
	menu["data"] = data
	c.JSON(http.StatusOK, menu)
}

func GetStructure(c *gin.Context) {
	schools, err := listSchools()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0)
	for _, school := range schools {
		grades, _ := listGrades(school)
		gradeNodes := make([]gin.H, 0)
		for _, grade := range grades {
			classes, _ := listClasses(school, grade)
			classNodes := make([]gin.H, 0)
			for _, classNumber := range classes {
				classNodes = append(classNodes, gin.H{"text": classNumber, "children": nil})
			}
			gradeNodes = append(gradeNodes, gin.H{"text": grade, "children": classNodes})
		}
		out = append(out, gin.H{"text": school, "children": gradeNodes})
	}
	c.JSON(http.StatusOK, out)
}

func GetAutorunStatus(c *gin.Context) {
	_, _ = db.RefreshAutorunStatuses(time.Now())
	rows, err := db.FetchAutorunRecords("")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, mapAutorunRecord(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func GetAutorunHashStatus(c *gin.Context) {
	hashid := c.Param("hashid")
	_, _ = db.RefreshAutorunStatuses(time.Now())
	rows, err := db.FetchAutorunRecords(hashid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(rows) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": mapAutorunRecord(rows[0])})
}

func DeleteAutorunRecord(c *gin.Context) {
	hashid := c.Param("hashid")
	affected, err := db.DeleteAutorunRecord(hashid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "记录不存在"})
		return
	}
	_, _ = db.RefreshAutorunStatuses(time.Now())
	c.JSON(http.StatusOK, gin.H{"status": 200, "deleted": affected, "id": hashid})
}

func PutCompensationRule(c *gin.Context) {
	var payload autorunPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数: " + err.Error()})
		return
	}
	if payload.Type != 0 {
		payload.Type = 0
	}
	dateStr, _ := payload.Content["date"].(string)
	useDateStr, _ := payload.Content["useDate"].(string)
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数: date 格式错误"})
		return
	}
	if _, err := time.Parse("2006-01-02", useDateStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数: useDate 格式错误"})
		return
	}
	params := map[string]interface{}{"rule": map[string]interface{}{"date": dateStr, "useDate": useDateStr}}
	scope := parseScopeInput(payload.Scope)
	hashID := makeHashID(payload.Type, scope, payload.Priority, params)
	record := dbTable.AutorunRecord{HashID: hashID, EType: payload.Type, Scope: scope, Parameters: params, Level: payload.Priority, Status: 0}
	if err := db.UpsertAutorunRecord(&record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_, _ = db.RefreshAutorunStatuses(time.Now())
	c.JSON(http.StatusOK, gin.H{"status": 200, "id": hashID})
}

func PutTimetableRule(c *gin.Context) {
	var payload autorunPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数: " + err.Error()})
		return
	}
	payload.Type = 1
	dateStr, _ := payload.Content["date"].(string)
	timetableID, _ := payload.Content["timetableId"].(string)
	if timetableID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数: timetableId 必须为非空字符串"})
		return
	}
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数: date 格式错误"})
		return
	}
	params := map[string]interface{}{"rule": map[string]interface{}{"date": dateStr, "timetableId": timetableID}}
	scope := parseScopeInput(payload.Scope)
	hashID := payload.ID
	if hashID == "" {
		if idInContent, ok := payload.Content["id"].(string); ok {
			hashID = idInContent
		}
	}
	if hashID == "" {
		hashID = makeHashID(payload.Type, scope, payload.Priority, params)
	}
	record := dbTable.AutorunRecord{HashID: hashID, EType: payload.Type, Scope: scope, Parameters: params, Level: payload.Priority, Status: 0}
	if err := db.UpsertAutorunRecord(&record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_, _ = db.RefreshAutorunStatuses(time.Now())
	c.JSON(http.StatusOK, gin.H{"status": 200, "id": hashID})
}

func PutScheduleRule(c *gin.Context) {
	var payload autorunPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数: " + err.Error()})
		return
	}
	payload.Type = 2
	dateStr, _ := payload.Content["date"].(string)
	scheduleObj, ok := payload.Content["schedule"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "content.schedule 必须为对象"})
		return
	}
	if _, ok := scheduleObj["periods"].([]interface{}); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "content.schedule.periods 必须为数组"})
		return
	}
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数: date 格式错误"})
		return
	}
	params := map[string]interface{}{"rule": map[string]interface{}{"date": dateStr, "schedule": map[string]interface{}{"periods": scheduleObj["periods"]}}}
	scope := parseScopeInput(payload.Scope)
	hashID := makeHashID(payload.Type, scope, payload.Priority, params)
	record := dbTable.AutorunRecord{HashID: hashID, EType: payload.Type, Scope: scope, Parameters: params, Level: payload.Priority, Status: 0}
	if err := db.UpsertAutorunRecord(&record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_, _ = db.RefreshAutorunStatuses(time.Now())
	c.JSON(http.StatusOK, gin.H{"status": 200, "id": hashID})
}

func PutAllRule(c *gin.Context) {
	var payload autorunPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数: " + err.Error()})
		return
	}
	payload.Type = 3
	dateStr, _ := payload.Content["date"].(string)
	timetableID, _ := payload.Content["timetableId"].(string)
	if timetableID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "content.timetableId 必须为非空字符串"})
		return
	}
	scheduleObj, ok := payload.Content["schedule"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "content.schedule 必须为对象"})
		return
	}
	if _, ok := scheduleObj["periods"].([]interface{}); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "content.schedule.periods 必须为数组"})
		return
	}
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效参数: date 格式错误"})
		return
	}
	params := map[string]interface{}{"rule": map[string]interface{}{"date": dateStr, "timetableId": timetableID, "schedule": map[string]interface{}{"periods": scheduleObj["periods"]}}}
	scope := parseScopeInput(payload.Scope)
	hashID := makeHashID(payload.Type, scope, payload.Priority, params)
	record := dbTable.AutorunRecord{HashID: hashID, EType: payload.Type, Scope: scope, Parameters: params, Level: payload.Priority, Status: 0}
	if err := db.UpsertAutorunRecord(&record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_, _ = db.RefreshAutorunStatuses(time.Now())
	c.JSON(http.StatusOK, gin.H{"status": 200, "id": hashID})
}

func CompensationFromHoliday(c *gin.Context) {
	year := c.Param("year")
	month := c.Param("month")
	day := c.Param("day")

	dateStr := fmt.Sprintf("%s-%s-%s", year, month, day)
	workday, exists := valence.CompensationFromHoliday(dateStr)

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"detail": "该日期不是调休休息日"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"holiday": dateStr,
		"workday": workday,
	})
}

func CompensationFromWorkday(c *gin.Context) {
	year := c.Param("year")
	month := c.Param("month")
	day := c.Param("day")

	dateStr := fmt.Sprintf("%s-%s-%s", year, month, day)
	holiday, exists := valence.CompensationFromWorkday(dateStr)

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"detail": "该日期不是补班日"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workday": dateStr,
		"holiday": holiday,
	})
}

func CompensationFromYear(c *gin.Context) {
	yearStr := c.Param("year")
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "年份参数格式错误"})
		return
	}

	pairs := valence.CompensationPairs(year)

	result := make([]gin.H, len(pairs))
	for idx, p := range pairs {
		result[idx] = gin.H{
			"holiday": p.Holiday,
			"workday": p.Workday,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"year":  year,
		"count": len(pairs),
		"pairs": result,
	})
}

func parseScope(scope string) (string, string, string, bool) {
	parts := strings.Split(scope, "/")
	if len(parts) < 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func GetScheduleByDate(c *gin.Context) {
	dateStr := c.Query("date")
	scope := c.Query("scope")
	dateObj, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的日期格式，应为 YYYY-MM-DD"})
		return
	}
	school, grade, classNumber, ok := parseScope(scope)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 scope 或配置缺失"})
		return
	}
	schedule := db.GetSchedule(school, grade, classNumber)
	timetable := db.GetTimetable(school, grade)
	_, _ = db.RefreshAutorunStatuses(time.Now())
	records, _ := db.FetchAutorunRecords("")
	resolved := service.ApplyScheduleRules(schedule.DailyClasses, timetable.Timetable, records, school, grade, classNumber, dateObj)
	periods := service.BuildPeriodsForDate(resolved, timetable.Timetable, dateObj)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"periods": periods}})
}

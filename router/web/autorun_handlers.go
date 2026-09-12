package web

import (
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/middleware"
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/service"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	invalidArgPrefix = "无效参数: "
	dateLayout       = "2006-01-02"
)

// clientConfigSettingKeys 自动任务可以覆盖的桌面端本地配置项（白名单）
var clientConfigSettingKeys = map[string]bool{
	"isWindowAlwaysOnTop":    true, // 窗口置顶
	"isDuringClassHidden":    true, // 上课隐藏
	"isAlwaysMinimized":      true, // 始终缩小
	"isDuringClassCountdown": true, // 课上计时
}

func badRequestDetail(c *gin.Context, detail string) {
	c.JSON(http.StatusBadRequest, gin.H{"detail": detail})
}

func badRequestInvalidArg(c *gin.Context, detail string) {
	badRequestDetail(c, invalidArgPrefix+detail)
}

func isValidDate(value string) bool {
	_, err := time.Parse(dateLayout, value)
	return err == nil
}

func validateDateField(c *gin.Context, fieldName string, value string) bool {
	if !isValidDate(value) {
		badRequestInvalidArg(c, fieldName+" 格式错误")
		return false
	}
	return true
}

// validateScheduleAction 校验课程表内容（SCHEDULE / ALL 共用）
func validateScheduleAction(action map[string]interface{}) string {
	scheduleObj, ok := action["schedule"].(map[string]interface{})
	if !ok {
		return "schedule 必须为对象"
	}
	if _, ok := scheduleObj["periods"].([]interface{}); !ok {
		return "schedule.periods 必须为数组"
	}
	return ""
}

// validateClientConfigAction 校验客户端配置内容
func validateClientConfigAction(action map[string]interface{}) string {
	settings, ok := action["settings"].(map[string]interface{})
	if !ok || len(settings) == 0 {
		return "settings 必须为非空对象"
	}
	for key, value := range settings {
		if !clientConfigSettingKeys[key] {
			return "settings 含不支持的配置项: " + key
		}
		if _, ok := value.(bool); !ok {
			return "settings." + key + " 必须为布尔值"
		}
	}
	return ""
}

// validateEntryAction 校验条目内容，返回错误详情（空串表示通过）
func validateEntryAction(etype int, action map[string]interface{}) string {
	if len(action) == 0 {
		return "action 不能为空"
	}
	switch etype {
	case dbTable.AutorunTypeCompensation:
		useDate, _ := action["useDate"].(string)
		if !isValidDate(useDate) {
			return "useDate 格式错误"
		}
	case dbTable.AutorunTypeTimetable:
		if id, _ := action["timetableId"].(string); id == "" {
			return "timetableId 必须为非空字符串"
		}
	case dbTable.AutorunTypeSchedule:
		return validateScheduleAction(action)
	case dbTable.AutorunTypeAll:
		if id, _ := action["timetableId"].(string); id == "" {
			return "timetableId 必须为非空字符串"
		}
		return validateScheduleAction(action)
	case dbTable.AutorunTypeClientConfig:
		return validateClientConfigAction(action)
	default:
		return "type 必须为 0-" + strconv.Itoa(dbTable.AutorunTypeClientConfig)
	}
	return ""
}

// validateEntryCondition 校验生效条件，返回错误详情（空串表示通过）。
// etype 用于限制只在客户端求值的条件（时刻事件）不被课表类任务使用。
func validateEntryCondition(when *dbTable.AutorunCondition, etype int) string {
	if when == nil {
		return ""
	}
	for _, day := range when.Weekdays {
		if day < 0 || day > 6 {
			return "when.weekdays 取值必须为 0-6"
		}
	}
	switch when.Kind {
	case dbTable.AutorunWhenDate:
		if !isValidDate(when.Date) {
			return "when.date 格式错误"
		}
	case dbTable.AutorunWhenRange:
		if !isValidDate(when.StartDate) {
			return "when.startDate 格式错误"
		}
		// endDate 可省略：表示从 startDate 起长期生效
		if when.EndDate != "" {
			if !isValidDate(when.EndDate) {
				return "when.endDate 格式错误"
			}
			if when.StartDate > when.EndDate {
				return "when.startDate 不能晚于 when.endDate"
			}
		}
	case dbTable.AutorunWhenWeekly:
		if when.EveryWeeks <= 0 {
			return "when.everyWeeks 必须为正整数"
		}
		if when.WeekOffset < 0 || when.WeekOffset >= when.EveryWeeks {
			return "when.weekOffset 必须落在 0 到 everyWeeks-1 之间"
		}
		if when.StartDate != "" && !isValidDate(when.StartDate) {
			return "when.startDate 格式错误"
		}
		if when.EndDate != "" && !isValidDate(when.EndDate) {
			return "when.endDate 格式错误"
		}
	case dbTable.AutorunWhenEvent:
		// 时刻事件只由桌面端本地求值：课表类任务在服务端解析，无法判定节次时刻，直接拒绝以免误解为「全天生效」
		if etype != dbTable.AutorunTypeClientConfig {
			return "when.kind=event 仅支持客户端配置类型"
		}
		if when.Event == dbTable.AutorunEventClassStart || when.Event == dbTable.AutorunEventClassEnd {
			if when.Period <= 0 {
				return "when.period 必须为正整数"
			}
			return ""
		}
		if when.Event != dbTable.AutorunEventStartup {
			return "when.event 不受支持"
		}
	case dbTable.AutorunWhenCron:
		if !service.IsValidCron(when.Cron) {
			return "when.cron 不是合法的 5 字段表达式"
		}
		if when.Duration < 0 {
			return "when.duration 不能为负数"
		}
	default:
		return "when.kind 不受支持"
	}
	return ""
}

// checkAutorunScope 校验作用域写权限（新建作用域与旧作用域都必须在权限内），失败时已写入响应
func checkAutorunScope(c *gin.Context, scopes ...[]string) bool {
	for _, list := range scopes {
		for _, s := range list {
			if !middleware.CheckUserScopeString(c, s) {
				return false
			}
		}
	}
	return true
}

// loadAutorunRecord 读取既有任务，用于取回旧作用域与创建时间
func loadAutorunRecord(hashID string) (dbTable.AutorunRecord, bool) {
	if hashID == "" {
		return dbTable.AutorunRecord{}, false
	}
	rows, err := db.FetchAutorunRecords(hashID)
	if err != nil || len(rows) == 0 {
		return dbTable.AutorunRecord{}, false
	}
	return rows[0], true
}

// saveAutorunRecord 落库并刷新状态、广播刷新（失败时已写入响应）
func saveAutorunRecord(c *gin.Context, record dbTable.AutorunRecord, oldScopes []string) bool {
	if err := db.UpsertAutorunRecord(&record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return false
	}
	_, _ = db.RefreshAutorunStatuses(time.Now())
	// 规则变更影响课表解析，按新旧作用域并集广播刷新
	broadcastScopes(mergeScopes(oldScopes, record.Scope))
	c.JSON(http.StatusOK, gin.H{"status": 200, "id": record.HashID})
	return true
}

// persistAutorunRule 兼容 v1 的按类型写入：内容被包装成任务内的唯一一条条目
func persistAutorunRule(c *gin.Context, payload autorunPayload, params map[string]interface{}, hashID string) {
	scope := parseScopeInput(payload.Scope)
	if !checkAutorunScope(c, scope) {
		return
	}
	existing, found := loadAutorunRecord(hashID)
	if hashID == "" {
		hashID = makeHashID(payload.Type, scope, payload.Priority, params)
		existing, found = loadAutorunRecord(hashID)
	}
	if found && !checkAutorunScope(c, existing.Scope) {
		return
	}
	record := dbTable.AutorunRecord{
		HashID:     hashID,
		EType:      payload.Type,
		Scope:      scope,
		Parameters: params,
		Level:      payload.Priority,
		Status:     0,
	}
	if found {
		record.CreatedAt = existing.CreatedAt
	}
	saveAutorunRecord(c, record, existing.Scope)
}

// PutAutorunTask 统一任务写入接口：一个任务携带若干条目（单日/范围/每周轮换/事件/cron）
func PutAutorunTask(c *gin.Context) {
	var payload autorunTaskPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		badRequestInvalidArg(c, err.Error())
		return
	}
	if payload.Type < 0 || payload.Type > dbTable.AutorunTypeClientConfig {
		badRequestInvalidArg(c, "type 必须为 0-"+strconv.Itoa(dbTable.AutorunTypeClientConfig))
		return
	}
	if len(payload.Entries) == 0 {
		badRequestInvalidArg(c, "entries 不能为空")
		return
	}
	entries := make([]dbTable.AutorunEntry, 0, len(payload.Entries))
	for i, input := range payload.Entries {
		if detail := validateEntryAction(payload.Type, input.Action); detail != "" {
			badRequestDetail(c, fmt.Sprintf("entries[%d]: %s", i, detail))
			return
		}
		if detail := validateEntryCondition(input.When, payload.Type); detail != "" {
			badRequestDetail(c, fmt.Sprintf("entries[%d]: %s", i, detail))
			return
		}
		entries = append(entries, dbTable.AutorunEntry{
			ID:       entryID(input.ID, i),
			Disabled: input.Enabled != nil && !*input.Enabled,
			Note:     input.Note,
			When:     input.When,
			Action:   input.Action,
		})
	}
	scope := parseScopeInput(payload.Scope)
	if !checkAutorunScope(c, scope) {
		return
	}
	existing, found := loadAutorunRecord(payload.ID)
	if found && !checkAutorunScope(c, existing.Scope) {
		return
	}
	hashID := payload.ID
	if hashID == "" {
		hashID = makeTaskHashID(payload.Type, scope, payload.Priority, payload.Name, entries)
		existing, found = loadAutorunRecord(hashID)
	}
	record := dbTable.AutorunRecord{
		HashID:     hashID,
		Name:       payload.Name,
		EType:      payload.Type,
		Scope:      scope,
		Disabled:   payload.Enabled != nil && !*payload.Enabled,
		Entries:    entries,
		Parameters: map[string]interface{}{},
		Level:      payload.Priority,
		Status:     0,
	}
	if found {
		record.CreatedAt = existing.CreatedAt
	}
	saveAutorunRecord(c, record, existing.Scope)
}

func entryID(raw string, index int) string {
	if raw != "" {
		return raw
	}
	return "e" + strconv.Itoa(index+1)
}

// entryContentOut 导出条目内容；v1 的单日规则把日期并回内容，保持旧契约不变
func entryContentOut(entry dbTable.AutorunEntry) map[string]interface{} {
	content := make(map[string]interface{}, len(entry.Action))
	for k, v := range entry.Action {
		content[k] = v
	}
	if entry.When != nil && (entry.When.Kind == "" || entry.When.Kind == dbTable.AutorunWhenDate) && entry.When.Date != "" {
		content["date"] = entry.When.Date
	}
	return content
}

func autorunTypeName(etype int) string {
	switch etype {
	case dbTable.AutorunTypeCompensation:
		return "COMPENSATION"
	case dbTable.AutorunTypeTimetable:
		return "TIMETABLE"
	case dbTable.AutorunTypeSchedule:
		return "SCHEDULE"
	case dbTable.AutorunTypeAll:
		return "ALL"
	case dbTable.AutorunTypeClientConfig:
		return "CLIENT_CONFIG"
	default:
		return strconv.Itoa(etype)
	}
}

func autorunStatusText(status int) string {
	switch status {
	case 0:
		return "待生效"
	case 1:
		return "生效中"
	case 2:
		return "已过期"
	default:
		return "未知"
	}
}

func mapAutorunRecord(r dbTable.AutorunRecord) gin.H {
	entries := service.EntriesOf(r)
	content := map[string]interface{}{}
	entriesOut := make([]gin.H, 0, len(entries))
	for _, entry := range entries {
		entriesOut = append(entriesOut, gin.H{
			"id":      entry.ID,
			"enabled": !entry.Disabled,
			"note":    entry.Note,
			"when":    entry.When,
			"action":  entry.Action,
			"content": entryContentOut(entry),
		})
	}
	// v1 兼容：content 取首条条目的内容
	if len(entries) > 0 {
		content = entryContentOut(entries[0])
	}

	return gin.H{
		"id":       r.HashID,
		"name":     r.Name,
		"type":     autorunTypeName(r.EType),
		"priority": r.Level,
		"status":   autorunStatusText(r.Status),
		"enabled":  !r.Disabled,
		"scope":    r.Scope,
		"content":  content,
		"entries":  entriesOut,
	}
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
	// 删除前取回规则作用域，删除后按原作用域广播刷新
	var scopes []string
	if rows, err := db.FetchAutorunRecords(hashid); err == nil && len(rows) > 0 {
		scopes = rows[0].Scope
	}
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
	broadcastScopes(scopes)
	c.JSON(http.StatusOK, gin.H{"status": 200, "deleted": affected, "id": hashid})
}

// bindAutorunContent 读取 v1 载荷的 content 字段
func bindAutorunContent(c *gin.Context) (autorunPayload, map[string]interface{}, bool) {
	var payload autorunPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		badRequestInvalidArg(c, err.Error())
		return payload, nil, false
	}
	if payload.Content == nil {
		payload.Content = map[string]interface{}{}
	}
	return payload, payload.Content, true
}

func PutCompensationRule(c *gin.Context) {
	payload, content, ok := bindAutorunContent(c)
	if !ok {
		return
	}
	payload.Type = dbTable.AutorunTypeCompensation
	dateStr, _ := content["date"].(string)
	useDateStr, _ := content["useDate"].(string)
	if !validateDateField(c, "date", dateStr) {
		return
	}
	if !validateDateField(c, "useDate", useDateStr) {
		return
	}
	params := map[string]interface{}{"rule": map[string]interface{}{"date": dateStr, "useDate": useDateStr}}
	persistAutorunRule(c, payload, params, "")
}

func PutTimetableRule(c *gin.Context) {
	payload, content, ok := bindAutorunContent(c)
	if !ok {
		return
	}
	payload.Type = dbTable.AutorunTypeTimetable
	timetableID, _ := content["timetableId"].(string)
	if timetableID == "" {
		badRequestInvalidArg(c, "timetableId 必须为非空字符串")
		return
	}
	dateStr, _ := content["date"].(string)
	if !validateDateField(c, "date", dateStr) {
		return
	}
	params := map[string]interface{}{"rule": map[string]interface{}{"date": dateStr, "timetableId": timetableID}}
	hashID := payload.ID
	if hashID == "" {
		if idInContent, ok := content["id"].(string); ok {
			hashID = idInContent
		}
	}
	persistAutorunRule(c, payload, params, hashID)
}

func PutScheduleRule(c *gin.Context) {
	payload, content, ok := bindAutorunContent(c)
	if !ok {
		return
	}
	payload.Type = dbTable.AutorunTypeSchedule
	dateStr, _ := content["date"].(string)
	if detail := validateScheduleAction(content); detail != "" {
		badRequestDetail(c, detail)
		return
	}
	if !validateDateField(c, "date", dateStr) {
		return
	}
	scheduleObj := content["schedule"].(map[string]interface{})
	params := map[string]interface{}{"rule": map[string]interface{}{"date": dateStr, "schedule": map[string]interface{}{"periods": scheduleObj["periods"]}}}
	persistAutorunRule(c, payload, params, "")
}

func PutAllRule(c *gin.Context) {
	payload, content, ok := bindAutorunContent(c)
	if !ok {
		return
	}
	payload.Type = dbTable.AutorunTypeAll
	dateStr, _ := content["date"].(string)
	timetableID, _ := content["timetableId"].(string)
	if timetableID == "" {
		badRequestDetail(c, "timetableId 必须为非空字符串")
		return
	}
	if detail := validateScheduleAction(content); detail != "" {
		badRequestDetail(c, detail)
		return
	}
	if !validateDateField(c, "date", dateStr) {
		return
	}
	scheduleObj := content["schedule"].(map[string]interface{})
	params := map[string]interface{}{"rule": map[string]interface{}{"date": dateStr, "timetableId": timetableID, "schedule": map[string]interface{}{"periods": scheduleObj["periods"]}}}
	persistAutorunRule(c, payload, params, "")
}

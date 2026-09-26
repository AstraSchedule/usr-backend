package web

import (
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/db"
	"AstraScheduleServerGo/router/client"

	"gorm.io/gorm"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

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

// makeHashID 生成自动任务规则的稳定哈希 ID。
// 安全修复：哈希输入加入 namespace，避免不同租户的相同规则产生相同 ID，
// 防止 upsert 时跨租户互相覆盖（主键 hash_id 不含 namespace 的隔离缺陷）
func makeHashID(ns string, etype int, scope []string, level int, parameters map[string]interface{}) string {
	sum := sha256.Sum256([]byte(ns + "|" + strconv.Itoa(etype) + "|" + strconv.Itoa(level) + "|" + stringsFromScope(scope) + "|" + stableMapString(parameters)))
	return hex.EncodeToString(sum[:])[:16]
}

// parseScopeInputStrict 解析写入用的 scope：格式非法时报错，绝不静默转成 ALL。
// 旧版 parseScopeInput 会把对象、数字、空数组、混入非字符串的数组一律变成 []string{"ALL"}，
// 于是「请求写错了」会变成「写出一条全站生效的规则」——这里必须显式拒绝。
// 仅 raw == nil（字段缺省）保留 ALL 这一文档化的默认值。
func parseScopeInputStrict(raw interface{}) ([]string, string) {
	switch v := raw.(type) {
	case nil:
		return []string{"ALL"}, ""
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, "scope 不能为空字符串"
		}
		return []string{trimmed}, ""
	case []string:
		return normalizeScopeEntries(v)
	case []interface{}:
		list := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, "scope 只能由字符串组成"
			}
			list = append(list, s)
		}
		return normalizeScopeEntries(list)
	default:
		return nil, "scope 必须为字符串或字符串数组"
	}
}

func normalizeScopeEntries(list []string) ([]string, string) {
	if len(list) == 0 {
		return nil, "scope 不能为空数组"
	}
	out := make([]string, 0, len(list))
	for _, raw := range list {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return nil, "scope 不能包含空字符串"
		}
		out = append(out, trimmed)
	}
	return out, ""
}

// scopeInput 解析自动任务的 scope 入参：
//   - 字段缺省（未出现在请求体里）→ 默认 ALL；
//   - 显式 null → 400：语义上「清空作用域」不应被当成「默认全部」，否则会静默写入全局作用域；
//   - 其余情况交给 parseScopeInputStrict 校验。
func scopeInput(raw json.RawMessage) ([]string, string) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return parseScopeInputStrict(nil)
	}
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, "scope 不能为 null"
	}
	var value interface{}
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, "scope 必须为字符串或字符串数组"
	}
	return parseScopeInputStrict(value)
}

// makeTaskHashID 生成 v2 任务的稳定哈希 ID（命名空间 + 类型 + 优先级 + 作用域 + 名称 + 条目内容）。
// 用 JSON 编码而不是手工拼接：JSON 对 map 键排序，且字段边界明确，
// 不会出现 "a=b|c=d" 与 "a=b|c=d" 这类手工拼接导致的歧义碰撞。
// 哈希输入必须包含 namespace：否则不同租户的相同任务会得到同一个 hash_id，
// 而 Upsert 以 hash_id 为冲突键 + UpdateAll，会跨租户互相覆盖。
func makeTaskHashID(ns string, etype int, scope []string, level int, name string, entries []dbTable.AutorunEntry) string {
	payload := struct {
		Namespace string                 `json:"namespace"`
		Type      int                    `json:"type"`
		Level     int                    `json:"level"`
		Scope     []string               `json:"scope"`
		Name      string                 `json:"name"`
		Entries   []dbTable.AutorunEntry `json:"entries"`
	}{
		Namespace: ns,
		Type:      etype,
		Level:     level,
		Scope:     sortedScope(scope),
		Name:      name,
		Entries:   entries,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		// 条目内容均来自 JSON 请求体，理论上不会出错；退化为不含条目的哈希保证仍然稳定
		raw = []byte(strconv.Itoa(etype) + "|" + strconv.Itoa(level) + "|" + stringsFromScope(scope) + "|" + name)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:16]
}

func sortedScope(scope []string) []string {
	copyScope := append([]string(nil), scope...)
	sort.Strings(copyScope)
	return copyScope
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

func parseClassList(input dbTable.ClassList) dbTable.ClassList {
	if len(input) == 0 {
		return dbTable.ClassList{}
	}
	out := make(dbTable.ClassList, 0, len(input))
	for _, item := range input {
		if len(item) == 0 {
			out = append(out, []string{""})
			continue
		}
		out = append(out, item)
	}
	return out
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

func parseScope(scope string) (string, string, string, bool) {
	parts := strings.Split(scope, "/")
	if len(parts) < 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// mergeScopes 合并去重新旧作用域（更新规则时旧作用域的客户端同样需要刷新通知）
func mergeScopes(oldScopes, newScopes []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(oldScopes)+len(newScopes))
	for _, s := range append(oldScopes, newScopes...) {
		key := strings.TrimSpace(s)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

// broadcastScopes 在指定租户 namespace 内按作用域列表向在线客户端广播 SyncConfig
// （仅 WebSocket 模式生效，serverless 自动跳过）。支持 ALL / school / school/grade 粒度；返回成功发送条数。
func broadcastScopes(ns string, scopes []string) int {
	return client.BroadcastScopes(ns, scopes)
}

// bumpDataVersionForDeletedScopes 为「删除操作」推进数据版本。
// 作用域精确到班级时只推进该班；年级/学校/ALL 这类更粗的作用域用全局版本兜底
// （DataVersion 的粒度是班级，粗作用域无法一一枚举到具体班级）。
// 删除此时已经提交，版本推进失败无法回滚，因此只记录告警：让缓存多等一轮，
// 也好过把一次已经生效的删除报成失败。
func bumpDataVersionForDeletedScopes(conn *gorm.DB, namespace string, scopes []string) {
	now := time.Now()
	bumped := make(map[string]struct{})
	for _, raw := range scopes {
		parts := strings.Split(strings.TrimSpace(raw), "/")
		school, grade, class := "", "", ""
		if len(parts) >= 3 && parts[0] != "" && parts[1] != "" && parts[2] != "" {
			school, grade, class = parts[0], parts[1], parts[2]
		}
		key := school + "/" + grade + "/" + class
		if _, done := bumped[key]; done {
			continue
		}
		bumped[key] = struct{}{}
		if err := db.BumpDataVersion(conn, namespace, school, grade, class, now); err != nil {
			logrus.Warnf("推进数据版本失败（删除后缓存可能滞后）: scope=%q err=%v", raw, err)
		}
	}
}

// deleteWithVersionBump 在同一个事务里执行删除与版本推进。
// 删除已生效而版本写入失败时，缓存会永久停留在旧版本，因此两者必须原子。
// 返回删除行数：0 表示记录不存在（调用方回 404）。
func deleteWithVersionBump(namespace string, scopes []string, remove func(tx *gorm.DB) (int64, error)) (int64, error) {
	tx := db.GetDB().Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}
	affected, err := remove(tx)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	if affected == 0 {
		tx.Rollback()
		return 0, nil
	}
	bumpDataVersionForDeletedScopes(tx, namespace, scopes)
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return affected, nil
}

// purgeScopesHeader 声明本次写入让哪些班级的版本缓存失效（边缘据此删 KV）。
// 值是逗号分隔的 school/grade/class；边缘读不到或读不懂时不做任何操作。
const purgeScopesHeader = "X-Astra-Purge-Scopes"

// setPurgeScopes 在响应头里声明本次写入的失效范围。
// 必须在写响应体（c.JSON）之前调用——响应一旦开始写出，头就改不动了。
// 空列表不设置该头：没有失效声明等价于「这次写入与缓存无关」。
func setPurgeScopes(c *gin.Context, scopes []string) {
	cleaned := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if trimmed := strings.TrimSpace(scope); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) == 0 {
		return
	}
	c.Header(purgeScopesHeader, strings.Join(cleaned, ","))
}

// purgeScope 拼一个班级作用域（与源站 scope 的字面格式一致）
func purgeScope(school, grade, class string) string {
	return strings.Join([]string{school, grade, class}, "/")
}

// purgeScopesOfGrade 把年级级改动展开成该年级下所有班级的作用域：
// 边缘的 KV 键是班级粒度，年级级写入必须逐班声明，否则缓存不会失效。
func purgeScopesOfGrade(school, grade string) []string {
	classes := make([]string, 0)
	if err := db.GetDB().Model(&dbTable.Schedule{}).
		Where("school = ? AND grade = ?", school, grade).
		Pluck("class", &classes).Error; err != nil {
		return nil
	}
	scopes := make([]string, 0, len(classes))
	for _, class := range classes {
		if class == "" {
			continue
		}
		scopes = append(scopes, purgeScope(school, grade, class))
	}
	return scopes
}

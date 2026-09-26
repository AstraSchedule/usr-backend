package client

import "strings"

// BroadcastScopes 按生效域广播配置刷新（多租户版）：scope 形如 "学校/年级[/班级]"，空串或 ALL 表示该命名空间全量。
// 自动任务的手动清理与后台清理都走这里，避免两处各写一份 scope 解析。
func BroadcastScopes(namespace string, scopes []string) int {
	total := 0
	for _, raw := range scopes {
		// 统一使用规范化后的 scope：整体与各分段都先 TrimSpace，避免 " ALL " / " s / g " 无法匹配
		scope := strings.TrimSpace(raw)
		parts := strings.Split(scope, "/")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		switch {
		case scope == "" || strings.EqualFold(scope, "ALL"):
			total += BroadcastSyncAll(namespace)
		case len(parts) >= 2 && parts[0] != "" && parts[1] != "":
			total += BroadcastSync(namespace, parts[0], parts[1])
		case len(parts) == 1 && parts[0] != "":
			total += BroadcastSyncSchool(namespace, parts[0])
		}
	}
	return total
}

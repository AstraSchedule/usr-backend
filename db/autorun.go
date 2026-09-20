package db

import (
	"AstraScheduleServerGo/model/dbTable"
	"AstraScheduleServerGo/service"
	"time"

	"gorm.io/gorm/clause"
)

const hashIDWhere = "hash_id = ?"

// FetchAutorunRecords 获取自动任务记录（无命名空间，向后兼容）
func FetchAutorunRecords(hashid string) ([]dbTable.AutorunRecord, error) {
	return FetchAutorunRecordsNs("", hashid)
}

// FetchAutorunRecordsNs 获取自动任务记录（带命名空间）
// 安全修复：namespace 为空时不查询，避免跨租户返回数据
func FetchAutorunRecordsNs(namespace, hashid string) ([]dbTable.AutorunRecord, error) {
	records := make([]dbTable.AutorunRecord, 0)
	if namespace == "" {
		return records, nil
	}
	q := GetDB().Model(&dbTable.AutorunRecord{}).Where("namespace = ?", namespace)
	if hashid != "" {
		q = q.Where(hashIDWhere, hashid)
	}
	err := q.Find(&records).Error
	return records, err
}

// FetchAutorunRecordNamespacesByHash 按 hash_id 跨 namespace 查询记录所属的命名空间。
// 仅用于写入前的归属校验：主键是全局唯一的 hash_id，客户端若自带一个已属于其它租户的 ID，
// Upsert(UpdateAll) 会连 Namespace 一起改写，必须在校验后拒绝。
func FetchAutorunRecordNamespacesByHash(hashid string) ([]string, error) {
	out := make([]string, 0)
	if hashid == "" {
		return out, nil
	}
	err := GetDB().Model(&dbTable.AutorunRecord{}).
		Where(hashIDWhere, hashid).
		Pluck("namespace", &out).Error
	return out, err
}

// DeleteAutorunRecord 删除自动任务记录（无命名空间，向后兼容）
func DeleteAutorunRecord(hashid string) (int64, error) {
	return DeleteAutorunRecordNs("", hashid)
}

// DeleteAutorunRecordNs 删除自动任务记录（带命名空间）
// 安全修复：namespace 为空时不删除，避免跨租户删除
func DeleteAutorunRecordNs(namespace, hashid string) (int64, error) {
	if namespace == "" {
		return 0, nil
	}
	resp := GetDB().Where(hashIDWhere, hashid).Where("namespace = ?", namespace).Delete(&dbTable.AutorunRecord{})
	return resp.RowsAffected, resp.Error
}

func UpsertAutorunRecord(record *dbTable.AutorunRecord) error {
	return GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hash_id"}},
		UpdateAll: true,
	}).Create(record).Error
}

// deriveStatusForRecord 推导任务状态：0 待生效 / 1 生效中 / 2 已过期。
// v2 起按任务内所有条目的生效区间（并集）判定，v1 的单日规则退化为同一天内生效，结果不变。
func deriveStatusForRecord(record dbTable.AutorunRecord, today time.Time) int {
	if record.EType < dbTable.AutorunTypeCompensation || record.EType > dbTable.AutorunTypeMax {
		return 0
	}
	return service.TaskStatus(record, today)
}

// RefreshAutorunStatuses 刷新自动任务状态（无命名空间，向后兼容）
func RefreshAutorunStatuses(today time.Time) (int64, error) {
	return RefreshAutorunStatusesNs("", today)
}

// RefreshAutorunStatusesNs 刷新自动任务状态（带命名空间）
func RefreshAutorunStatusesNs(namespace string, today time.Time) (int64, error) {
	records, err := FetchAutorunRecordsNs(namespace, "")
	if err != nil {
		return 0, err
	}
	updated := int64(0)
	for i := range records {
		newStatus := deriveStatusForRecord(records[i], today)
		if newStatus == records[i].Status {
			continue
		}
		// status 只是给管理端列表展示的派生缓存，客户端响应由读取时按时间重新求值，
		// 不依赖该列；因此这里用 UpdateColumn 跳过时间戳维护——否则每次状态翻转都会推进
		// 自动任务记录的 UpdatedAt，进而推进课表版本，造成一次无意义的重新拉取
		if err := GetDB().Model(&dbTable.AutorunRecord{}).
			Where(hashIDWhere, records[i].HashID).
			UpdateColumn("status", newStatus).Error; err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

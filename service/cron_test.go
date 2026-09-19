package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidCron(t *testing.T) {
	valid := []string{"* * * * *", "0 8 * * 1", "*/15 6-22 * * 1-5", "0 0 1,15 * *", "30 7 * * 0"}
	for _, expr := range valid {
		assert.True(t, IsValidCron(expr), expr)
	}

	invalid := []string{"", "* * * *", "* * * * * *", "60 * * * *", "* 24 * * *", "* * 0 * *", "* * * 13 *", "* * * * 7", "a * * * *", "*/0 * * * *", "5-1 * * * *"}
	for _, expr := range invalid {
		assert.False(t, IsValidCron(expr), expr)
	}
}

func TestCronSpec_Match(t *testing.T) {
	spec, ok := ParseCron("30 8 * * 1")
	require.True(t, ok)

	// 2026-09-07 是周一
	assert.True(t, spec.match(time.Date(2026, time.September, 7, 8, 30, 0, 0, time.UTC)))
	assert.False(t, spec.match(time.Date(2026, time.September, 7, 8, 31, 0, 0, time.UTC)))
	assert.False(t, spec.match(time.Date(2026, time.September, 8, 8, 30, 0, 0, time.UTC)))
}

func TestCronSpec_PrevAndNext(t *testing.T) {
	spec, ok := ParseCron("0 8 * * 1")
	require.True(t, ok)

	// 2026-09-02 周二：上一次命中为 08-31 周一，下一次为 09-07 周一
	now := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	prev, ok := spec.Prev(now)
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, time.August, 31, 8, 0, 0, 0, time.UTC), prev)

	next, ok := spec.Next(now)
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, time.September, 7, 8, 0, 0, 0, time.UTC), next)
}

func TestCronSpec_PrevEarlierThanLimitHour(t *testing.T) {
	// 回归：limit 的小时早于命中小时时，仍应能找到更早一天/当天的命中
	spec, ok := ParseCron("0 8 * * *")
	require.True(t, ok)

	prev, ok := spec.Prev(time.Date(2026, time.September, 2, 7, 0, 0, 0, time.UTC))
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, time.September, 1, 8, 0, 0, 0, time.UTC), prev)
}

func TestCronSpec_StepAndRange(t *testing.T) {
	spec, ok := ParseCron("*/15 6-8 * * *")
	require.True(t, ok)

	assert.True(t, spec.match(time.Date(2026, time.September, 1, 6, 0, 0, 0, time.UTC)))
	assert.True(t, spec.match(time.Date(2026, time.September, 1, 8, 45, 0, 0, time.UTC)))
	assert.False(t, spec.match(time.Date(2026, time.September, 1, 9, 0, 0, 0, time.UTC)))
	assert.False(t, spec.match(time.Date(2026, time.September, 1, 6, 10, 0, 0, time.UTC)))
}

func TestCronSpec_NextKeepsLaterHitInSameHour(t *testing.T) {
	// 每小时的 0 分与 30 分命中：08:15 的下一次必须是 08:30，不能跳到 09:00
	spec, ok := ParseCron("0,30 * * * *")
	require.True(t, ok)

	next, ok := spec.Next(time.Date(2026, time.September, 1, 8, 15, 0, 0, time.UTC))
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, time.September, 1, 8, 30, 0, 0, time.UTC), next)
}

// DST 切换日必须按本地墙钟取命中时刻：不能用 day.Add(绝对时长)，否则本地 08:00 会变成 07:00/09:00
func TestCronSpec_DaylightSavingUsesWallClock(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("时区数据不可用，跳过 DST 用例")
	}
	spec, ok := ParseCron("0 8 * * *")
	require.True(t, ok)

	// 2026-03-08 是美国夏令时开始日（当地 02:00 跳到 03:00）
	next, ok := spec.Next(time.Date(2026, time.March, 8, 0, 0, 0, 0, location))
	require.True(t, ok)
	assert.Equal(t, 8, next.Hour(), "命中时刻应为本地 08:00")
	assert.Equal(t, 0, next.Minute())
	assert.Equal(t, 8, next.Day())

	// 2026-11-01 是夏令时结束日（当地 02:00 回到 01:00）
	prev, ok := spec.Prev(time.Date(2026, time.November, 1, 12, 0, 0, 0, location))
	require.True(t, ok)
	assert.Equal(t, 8, prev.Hour(), "命中时刻应为本地 08:00")
	assert.Equal(t, 0, prev.Minute())
}

func TestCronSpec_RareExpressionBeyondOneYear(t *testing.T) {
	// 2 月 29 日：相邻两次命中可能相隔 4 年（甚至 8 年），搜索窗口必须覆盖
	spec, ok := ParseCron("0 0 29 2 *")
	require.True(t, ok)

	// 2100 不是闰年，因此 2096-02-29 之后的下一次是 2104-02-29，间隔 8 年（> 366 天）
	prev, ok := spec.Prev(time.Date(2104, time.January, 1, 0, 0, 0, 0, time.UTC))
	require.True(t, ok, "8 年前的命中不应被搜索窗口漏掉")
	assert.Equal(t, time.Date(2096, time.February, 29, 0, 0, 0, 0, time.UTC), prev)

	next, ok := spec.Next(time.Date(2096, time.March, 1, 0, 0, 0, 0, time.UTC))
	require.True(t, ok, "世纪闰年规则下 8 年后的命中同样要能找到")
	assert.Equal(t, time.Date(2104, time.February, 29, 0, 0, 0, 0, time.UTC), next)
}

func TestCronSpec_DayOfMonthOrWeekday(t *testing.T) {
	// 日与周同时受限：标准 cron 取「或」
	spec, ok := ParseCron("0 0 1 * 1")
	require.True(t, ok)

	assert.True(t, spec.match(time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)), "1 号命中")
	assert.True(t, spec.match(time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC)), "周一命中")
	assert.False(t, spec.match(time.Date(2026, time.September, 8, 0, 0, 0, 0, time.UTC)), "非 1 号且非周一不命中")
}

// 2026-03-08 是美国夏令时开始日（当地 02:00 直接跳到 03:00）：
// 当天不存在的本地时刻按主流 cron 的「跳过」规则处理，当天不生效。
func TestCronSpec_SkipsNonexistentLocalTimeOnDSTStart(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("时区数据不可用，跳过 DST 用例")
	}
	dstDay := time.Date(2026, time.March, 8, 0, 0, 0, 0, location)

	missing, ok := ParseCron("30 2 * * *")
	require.True(t, ok)

	// 03-08 当天没有本地 02:30（time.Date 会把它规范化成 01:30）→ 跳过当天，命中落在次日
	next, ok := missing.Next(dstDay)
	require.True(t, ok)
	assert.Equal(t, 9, next.Day(), "跳变当天不应产生 02:30 命中")
	assert.Equal(t, 2, next.Hour())
	assert.Equal(t, 30, next.Minute())

	prev, ok := missing.Prev(time.Date(2026, time.March, 8, 12, 0, 0, 0, location))
	require.True(t, ok)
	assert.Equal(t, 7, prev.Day(), "跳变当天不应产生 02:30 命中")
	assert.Equal(t, 2, prev.Hour())
	assert.Equal(t, 30, prev.Minute())

	// 当天的 03:30 真实存在，正常命中
	existing, ok := ParseCron("30 3 * * *")
	require.True(t, ok)
	hit, ok := existing.Next(dstDay)
	require.True(t, ok)
	assert.Equal(t, 8, hit.Day())
	assert.Equal(t, 3, hit.Hour())
	assert.Equal(t, 30, hit.Minute())
}

// cronHitAt 只在时刻被 time.Date 规范化时报无效；秋季回拨日重复出现的时刻仍然有效
func TestCronHitAt_ValidityMarker(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("时区数据不可用，跳过 DST 用例")
	}
	dstDay := time.Date(2026, time.March, 8, 0, 0, 0, 0, location)

	_, ok := cronHitAt(dstDay, 2, 30)
	require.False(t, ok, "不存在的本地 02:30 应判为无效")
	_, ok = cronHitAt(dstDay, 3, 30)
	require.True(t, ok, "存在的本地 03:30 应判为有效")

	fallBack := time.Date(2026, time.November, 1, 0, 0, 0, 0, location)
	hit, ok := cronHitAt(fallBack, 1, 30)
	require.True(t, ok, "回拨日重复出现的 01:30 仍然有效")
	assert.Equal(t, 1, hit.Hour())
	assert.Equal(t, 30, hit.Minute())
}

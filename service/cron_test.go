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

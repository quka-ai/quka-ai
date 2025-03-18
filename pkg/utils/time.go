package utils

import "time"

func GetWeekStartAndEnd(date time.Time) (time.Time, time.Time) {
	// 获取本周一的日期
	weekday := date.Weekday()
	// 调整为一周的起始点为周一
	if weekday == time.Sunday {
		weekday = 7
	}
	// 计算周一的日期
	startOfWeek := date.AddDate(0, 0, -int(weekday)+1)
	// 将时间设置为0点0分0秒
	startOfWeek = time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, startOfWeek.Location())

	// 获取本周日的日期
	endOfWeek := startOfWeek.AddDate(0, 0, 6)
	// 将时间设置为23点59分59秒
	endOfWeek = time.Date(endOfWeek.Year(), endOfWeek.Month(), endOfWeek.Day(), 23, 59, 59, 0, endOfWeek.Location())

	return startOfWeek, endOfWeek
}


func GetMonthStartAndEnd(date time.Time) (time.Time, time.Time) {
	// 获取本月的第一天
	startOfMonth := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())

	// 计算下个月的第一天
	nextMonth := startOfMonth.AddDate(0, 1, 0)
	// 获取本月的最后一天
	endOfMonth := nextMonth.Add(-time.Second)

	return startOfMonth, endOfMonth
}
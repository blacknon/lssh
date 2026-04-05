// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func sumFloat64(values ...float64) (sum float64) {
	sum = 0.0
	for _, v := range values {
		sum += v
	}
	return sum
}

func uptimeFormatDuration(d time.Duration) string {
	// 総時間数を秒単位で取得
	totalSeconds := int64(d.Seconds())
	// 秒、分、時間、日の単位に変換
	seconds := totalSeconds % 60
	minutes := (totalSeconds / 60) % 60
	hours := (totalSeconds / 3600) % 24
	days := totalSeconds / (3600 * 24)

	return fmt.Sprintf("%4d[gray]days[none] %02d:%02d:%02d", days, hours, minutes, seconds)
}

func isEmptyStruct(data interface{}) bool {
	v := reflect.ValueOf(data)
	t := v.Type()

	// ポインタであれば、ポインタが指す型を取得
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// 構造体であり、フィールド数が0であるかどうかをチェック
	if t.Kind() == reflect.Struct && t.NumField() == 0 {
		return true
	}
	return false
}

func reverseSlice[T any](slice []T) {
	length := len(slice)
	for i := 0; i < length/2; i++ {
		slice[i], slice[length-1-i] = slice[length-1-i], slice[i]
	}
}

func CreatePercentGraph(length int, value, total float64, color string) string {
	// memory
	usedLength := int(value / total * float64(length))

	// Create Mem Bar
	bar := ""
	for i := 0; i < usedLength; i++ {
		bar += fmt.Sprintf("[%s]|", color)
	}
	for i := usedLength; i < length; i++ {
		bar += "[node] "
	}
	bar += "[none]"

	return bar
}

func maxInt(nums []int) int {
	max := nums[0]
	for _, num := range nums {
		if num > max {
			max = num
		}
	}
	return max
}

func maxFloat64(nums []float64) float64 {
	max := nums[0]
	for _, num := range nums {
		if num > max {
			max = num
		}
	}
	return max
}

func scaleMaxValue(maxValue float64) float64 {
	if maxValue == 0 {
		return 10
	}

	// 桁数に基づいてスケーリングする
	scale := math.Pow(10, math.Floor(math.Log10(maxValue)))
	return math.Ceil(maxValue/scale) * scale
}

func parseSize(size string) int64 {
	size = strings.ToUpper(strings.TrimSpace(size))
	var factor int64 = 1

	switch {
	case strings.HasSuffix(size, "KB"):
		factor = 1024
		size = strings.TrimSuffix(size, "KB")
	case strings.HasSuffix(size, "MB"):
		factor = 1024 * 1024
		size = strings.TrimSuffix(size, "MB")
	case strings.HasSuffix(size, "GB"):
		factor = 1024 * 1024 * 1024
		size = strings.TrimSuffix(size, "GB")
	case strings.HasSuffix(size, "TB"):
		factor = 1024 * 1024 * 1024 * 1024
		size = strings.TrimSuffix(size, "TB")
	}

	value, err := strconv.ParseFloat(strings.TrimSpace(size), 64)
	if err != nil {
		return 0
	}

	return int64(value * float64(factor))
}

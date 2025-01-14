package utils

import "strings"

// Contains 切片是否包含某个元素
func Contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

// ContainsSuffix 切片是否包含某个元素
func ContainsSuffix(slice []string, target string) bool {
	for _, s := range slice {

		if strings.HasSuffix(target, s) {
			return true
		}
	}
	return false
}

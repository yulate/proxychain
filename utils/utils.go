package utils

import (
	"log"
	"os"
	"strconv"
)

// FileExists 检查文件是否存在
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// ParsePort 将字符串类型的端口转换为整数
func ParsePort(portStr string) int {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Printf("转换端口失败: %v", err)
		return 0
	}
	return port
}

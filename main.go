package main

import (
	"embed"
	"fmt"
	_ "github.com/mattn/go-sqlite3" // 导入 SQLite 驱动
	"os"
	"proxychain/common"
	"proxychain/core"
)

const filePath = "data/czutf8.dat"

func main() {
	// 加载配置文件
	common.LoadConfig()

	// 加载ip数据库
	checkData()

	// 启动pipeline
	core.StartPipeline()
}

func checkData() {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Println("文件不存在, 释放中...")

		// 删除已经存在的文件夹，以防它已经被创建但权限不正确
		os.RemoveAll("data")

		// 创建文件夹并指定正确的权限
		err := os.Mkdir("data", 0755)
		if err != nil {
			fmt.Println("创建data文件夹失败：", err)
			return
		}

		// 再次检查文件夹的权限，并修复不正确的权限
		err = os.Chmod("data", 0755)
		if err != nil {
			fmt.Println("修正data文件夹权限失败：", err)
			return
		}

		ReleaseTheFile()
	}
}

//go:embed data/czutf8.dat
var embeddedData embed.FS

func ReleaseTheFile() {
	file, err := embeddedData.ReadFile("data/czutf8.dat")
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(filePath, file, 0644)
	if err != nil {
		fmt.Println("写入文件失败:", err)
		return
	}
}

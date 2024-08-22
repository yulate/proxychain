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
		err := os.Mkdir("data", 777)
		if err != nil {
			fmt.Println("创建data文件夹失败：", err)
		}
		ReleaseTheFile()
	}
}

//go:embed data/czutf8.dat
var data embed.FS

func ReleaseTheFile() {
	file, err := data.ReadFile("data/czutf8.dat")
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(filePath, []byte(file), 0644)
	if err != nil {
		fmt.Println("Failed to write file:", err)
		return
	}
}

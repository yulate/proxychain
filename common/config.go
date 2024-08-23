package common

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"proxychain/utils"
)

// Config 是用于存储配置的结构体
type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"server"`

	Database struct {
		Type string `yaml:"type"`
		Path string `yaml:"path"`
	} `yaml:"database"`

	Hunter struct {
		APIKey string `yaml:"apiKey"`
	} `yaml:"hunter"`

	Fofa struct {
		APIKey string `yaml:"apiKey"`
	} `yaml:"fofa"`

	Config struct {
		PreciseArea        bool   `yaml:"preciseArea"`        // 精确地区匹配
		OnlyChina          bool   `yaml:"onlyChina"`          // 只使用中国节点
		ObtainingProxyMode string `yaml:"obtainingProxyMode"` // 获取代理的模式
		TaskTime           int    `yaml:"taskTime"`           // 定时任务轮询间隔时间
		MiniProxyCount     int    `yaml:"miniProxyCount"`     // 数据库中最少代理数量的阈值
		PriorityDownNum    int    `yaml:"priorityDownNum"`    // 可信度每次减少的值
		PriorityUpNum      int    `yaml:"priorityUpNum"`      // 可信度每次增加的值
	}
}

// GlobalConfig 用于存储全局配置
var GlobalConfig Config

// LoadConfig 从yaml 配置文件中读取一些全局配置
func LoadConfig() {

	filePath := "./config.yaml"

	// 检测文件是否存在
	exists := utils.FileExists(filePath)
	if !exists {
		log.Fatal("config.yaml 配置文件不存在，终止退出程序！")
		os.Exit(1)
	}

	// 读取文件内容
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("无法读取配置文件: %v", err)
	}

	// 解析 YAML 内容
	err = yaml.Unmarshal(data, &GlobalConfig)
	if err != nil {
		log.Fatalf("无法解析配置文件: %v", err)
	}

	fmt.Printf("配置已加载: %+v\n", GlobalConfig)
}

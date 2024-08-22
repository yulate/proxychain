package core

import (
	"log"
	"proxychain/common"
	"proxychain/database"
	"proxychain/proxyPool"
	"time"
)

var (
	checkInterval = 1 * time.Minute // 定时任务的间隔时间
	minProxyCount = 50              // 数据库中最少代理数量的阈值
)

// startScheduledTasks 启动定时任务
func startScheduledTasks(ps *database.ProxyStorage) {

	checkInterval = time.Duration(common.GlobalConfig.Config.TaskTime) * time.Second
	minProxyCount = common.GlobalConfig.Config.MiniProxyCount

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("执行定时任务...")

			//loadProxies(ps)

			// 删除可信度 < 0 的代理
			err := ps.DeleteLowPriorityProxies()
			if err != nil {
				log.Printf("定时任务 - 删除可信度低于0的代理失败: %v\n", err)
			} else {
				log.Println("定时任务 - 成功删除可信度低于0的代理IP。")
			}

			// 检查代理可用性并更新优先级
			checkAndUpdateProxies(ps)

			// 检查数据库中的代理数量
			proxyCount, err := ps.GetProxyCount()
			if err != nil {
				log.Printf("定时任务 - 获取代理数量失败: %v\n", err)
			} else {
				log.Printf("定时任务 - 当前代理数量: %d\n", proxyCount)
				if proxyCount < minProxyCount {
					log.Println("定时任务 - 代理数量不足，开始获取新的代理...")
					proxyPool.GetProxyBase(ps)
				}
			}

			//统计中国和非中国代理的数量
			chinaCount, nonChinaCount, err := ps.GetCountryStatistics()
			if err != nil {
				log.Printf("定时任务 - 获取国家统计信息失败: %v\n", err)
			} else {
				log.Printf("定时任务 - 中国代理数量: %d, 非中国代理数量: %d\n", chinaCount, nonChinaCount)
			}

			loadProxies(ps)
		}
	}
}

package core

import (
	"log"
	"net"
	"proxychain/common"
	"proxychain/database"
	"proxychain/proxyPool"
	"proxychain/utils"
	"sync"
)

func StartPipeline() {
	// 初始化数据库，检测数据库是否存在
	dbPath := common.GlobalConfig.Database.Path
	if !utils.FileExists(dbPath) {
		log.Printf("数据库文件 %s 不存在，正在创建...\n", dbPath)
	}

	proxyStorage, err := database.NewProxyStorage(dbPath)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 启动定时任务
	go startScheduledTasks(proxyStorage)

	// 去除数据库中优先级低于0的ip
	err = proxyStorage.DeleteLowPriorityProxies()
	if err != nil {
		log.Printf("删除优先级低于0的代理失败: %v\n", err)
	} else {
		log.Println("成功删除优先级低于0的代理IP。")
	}

	// 并行执行耗时任务
	var wg sync.WaitGroup

	// 启动并行任务
	wg.Add(2)

	go func() {
		defer wg.Done()
		// 检测数据库是否有数据，如果有代理ip则进行检测，根据有效性更新优先级
		checkAndUpdateProxies(proxyStorage)
	}()

	//go func() {
	//	defer wg.Done()
	//	// 从 fofa 和 hunter 提取新的代理池数据
	//	proxyPool.GetProxyBase(proxyStorage)
	//}()

	// 立即加载代理，准备服务
	loadProxies(proxyStorage)

	// 启动本地代理，开放端口允许访问
	startProxy()

	// 等待所有耗时任务完成
	wg.Wait()
}

// checkAndUpdateProxies 检测更新代理有效性
func checkAndUpdateProxies(ps *database.ProxyStorage) {
	proxies, err := ps.GetActiveProxiesByPriority()
	if err != nil {
		log.Fatalf("获取代理失败: %v", err)
	}

	if len(proxies) == 0 {
		log.Println("数据库中没有可用的代理IP。")
		return
	}

	// 提取代理 URL 列表
	var proxyURLs []string
	for _, proxy := range proxies {
		proxyURLs = append(proxyURLs, proxy.URL)
	}

	// 执行并发检测
	targetURLs := []string{"https://www.google.com", "https://www.baidu.com", "http://www.baidu.com", "https://www.yulate.com", "https://www.ip138.com"}
	results := proxyPool.CheckProxy(proxyURLs, targetURLs)

	// 根据检测结果更新代理优先级
	for _, result := range results {
		if result.Success {
			log.Printf("代理 %s 可用，成功访问: %s\n", result.ProxyAddr, result.SuccessURL)
			ip, port, err := common.ExtractIPAndPort(result.ProxyAddr)
			if err != nil {
				log.Printf("解析代理地址失败: %v\n", err)
				continue
			}
			err = ps.IncreasePriority(ip, port)
			if err != nil {
				log.Printf("增加代理 %s 优先级失败: %v\n", result.ProxyAddr, err)
			}
		} else {
			log.Printf("代理 %s 不可用，降低优先级。\n", result.ProxyAddr)
			ip, port, err := common.ExtractIPAndPort(result.ProxyAddr)
			if err != nil {
				log.Printf("解析代理地址失败: %v\n", err)
				continue
			}
			err = ps.DecreasePriority(ip, port)
			if err != nil {
				log.Printf("降低代理 %s 优先级失败: %v\n", result.ProxyAddr, err)
			}
		}
	}
}

func startProxy() {
	listener, err := net.Listen("tcp", common.GlobalConfig.Server.Host+":"+common.GlobalConfig.Server.Port)
	if err != nil {
		log.Fatal("启动服务器失败:", err)
	}
	defer listener.Close()

	log.Println("代理服务器运行在" + common.GlobalConfig.Server.Host + ":" + common.GlobalConfig.Server.Port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("接受连接失败:", err)
			continue
		}
		go HandleConnection(conn)
	}
}

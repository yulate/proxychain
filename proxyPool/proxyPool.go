package proxyPool

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/kayon/iploc"
	"log"
	"math/rand"
	"net/http"
	"proxychain/common"
	"proxychain/database"
	"strings"
	"sync"
)

var (
	fofaURL      = "https://fofa.info/api/v1/search"
	hunterURL    = "https://hunter.qianxin.com/openApi"
	hunterAPIKey = ""
	fofaAPIKey   = ""
)

// GetProxyBase 初始化API密钥并开始获取代理池
func GetProxyBase(ps *database.ProxyStorage) {
	hunterAPIKey = common.GlobalConfig.Hunter.APIKey
	fofaAPIKey = common.GlobalConfig.Fofa.APIKey

	// 使用 WaitGroup 来并发获取代理
	var wg sync.WaitGroup

	if hunterAPIKey != "" {
		log.Println("开始获取Hunter代理池")
		wg.Add(1)
		go func() {
			defer wg.Done()
			getProxiesFromSource(ps, "hunter")
		}()
	}

	if fofaAPIKey != "" {
		log.Println("开始获取Fofa代理池")
		wg.Add(1)
		go func() {
			defer wg.Done()
			getProxiesFromSource(ps, "fofa")
		}()
	}

	// 等待所有并发操作完成
	wg.Wait()
}

// getProxiesFromSource 获取代理池数据并保存到数据库
func getProxiesFromSource(ps *database.ProxyStorage, source string) {
	var searchStatements []string
	var buildQueryURL func(string, int, int) string
	var processProxies func(string, *database.ProxyStorage)

	switch source {
	case "hunter":
		searchStatements = []string{
			`web.body="get all proxy from proxy pool"&&ip.country=="中国"`,
			`web.body="get all proxy from proxy pool"&&ip.country=="美国"`,
		}
		buildQueryURL = buildHunterQueryURL
		processProxies = processHunterProxies
	case "fofa":
		searchStatements = []string{
			`body="get all proxy from proxy pool"&&status_code="200"`,
		}
		buildQueryURL = buildFofaQueryURL
		processProxies = processFofaProxies
	}

	// 第一次少量请求，获取total num
	url := buildQueryURL(searchStatements[0], 1, 1)
	totalNum, err := getTotalNumber(url, source)
	if err != nil {
		log.Fatalf("获取 %s total num 失败：%v", source, err)
		return
	}

	if totalNum <= 0 {
		log.Printf("%s 返回的 total num 小于等于 0", source)
		return
	}

	pageSize := 10
	if source == "fofa" {
		pageSize = 40
	}
	totalPages := (totalNum + pageSize - 1) / pageSize

	// 在 0 到 totalPages 范围内生成一个随机页码
	randomPage := rand.Intn(totalPages) + 1

	var wg sync.WaitGroup
	for _, searchStatement := range searchStatements {
		queryURL := buildQueryURL(searchStatement, randomPage, pageSize)
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			processProxies(url, ps)
		}(queryURL)
	}

	// 等待所有并发操作完成
	wg.Wait()
}

// getTotalNumber 获取 Hunter 或 Fofa 数据的总数
func getTotalNumber(requestURL, source string) (int, error) {
	switch source {
	case "hunter":
		hunterResponse, err := fetchHunterResponse(requestURL)
		if err != nil {
			return 0, err
		}
		return hunterResponse.Data.Total, nil
	case "fofa":
		fofaResponse, err := fetchFofaResponse(requestURL)
		if err != nil {
			return 0, err
		}
		return fofaResponse.Size, nil
	default:
		return 0, fmt.Errorf("未知的数据源: %s", source)
	}
}

// buildHunterQueryURL 构建 Hunter 查询 URL
func buildHunterQueryURL(searchStatement string, page, pageSize int) string {
	encodedQuery := base64.URLEncoding.EncodeToString([]byte(searchStatement))
	return fmt.Sprintf("%s/search?api-key=%s&search=%s&page=%d&page_size=%d",
		hunterURL, hunterAPIKey, encodedQuery, page, pageSize)
}

// buildFofaQueryURL 构建 Fofa 查询 URL
func buildFofaQueryURL(searchStatement string, page, pageSize int) string {
	encodedQuery := base64.URLEncoding.EncodeToString([]byte(searchStatement))
	return fmt.Sprintf("%s/all?&key=%s&qbase64=%s&page=%d&size=%d",
		fofaURL, fofaAPIKey, encodedQuery, page, pageSize)
}

// processHunterProxies 获取代理列表，检查可用性，并保存到数据库
func processHunterProxies(requestURL string, ps *database.ProxyStorage) {
	proxyBases, err := fetchHunterData(requestURL)
	if err != nil {
		log.Fatalf("获取 Hunter 代理数据失败: %v", err)
	}

	var wg sync.WaitGroup
	for _, proxyBase := range proxyBases {
		wg.Add(1)
		go func(pb common.ProxyBase) {
			defer wg.Done()
			storeProxiesByBase(pb, ps)
		}(proxyBase)
	}

	// 等待所有并发操作完成
	wg.Wait()
}

// processFofaProxies 获取代理列表，检查可用性，并保存到数据库
func processFofaProxies(requestURL string, ps *database.ProxyStorage) {
	fofaData, err := fetchFofaData(requestURL)
	if err != nil {
		log.Fatalf("获取 Fofa 代理数据失败: %v", err)
	}

	var wg sync.WaitGroup
	for _, data := range fofaData {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			storeProxiesByFofa(addr, ps)
		}(data.FullAddress)
	}

	// 等待所有并发操作完成
	wg.Wait()
}

// storeProxiesByBase 根据 Hunter 返回的数据存储代理
func storeProxiesByBase(proxyBase common.ProxyBase, ps *database.ProxyStorage) {
	proxyList, err := GetProxyList(proxyBase.URL + "/all")
	if err != nil {
		log.Printf("获取代理列表失败: %v", err)
		return
	}
	checkAndStoreProxies(proxyList, proxyBase, ps)
}

// storeProxiesByFofa 根据 Fofa 返回的数据存储代理
func storeProxiesByFofa(proxyAddr string, ps *database.ProxyStorage) {
	if !strings.Contains(proxyAddr, "http") {
		proxyAddr = "http://" + proxyAddr
	}
	proxyList, err := GetProxyList(proxyAddr + "/all")
	if err != nil {
		log.Printf("获取代理列表失败: %v", err)
		return
	}
	checkAndStoreProxies(proxyList, common.ProxyBase{}, ps)
}

// checkAndStoreProxies 并发检测代理可用性并保存可用代理
func checkAndStoreProxies(proxyList []string, proxyBase common.ProxyBase, ps *database.ProxyStorage) {
	targetURLs := []string{"https://www.google.com", "https://www.baidu.com", "http://www.baidu.com", "https://www.yulate.com", "https://www.ip138.com"}
	results := CheckProxy(proxyList, targetURLs)

	var wg sync.WaitGroup
	for _, result := range results {
		wg.Add(1)
		go func(res common.ProxyCheckResult) {
			defer wg.Done()
			if res.Success {
				ip, port, err := common.ExtractIPAndPort(res.ProxyAddr)
				if err != nil {
					log.Printf("解析代理地址失败: %v\n", err)
					return
				}

				// 检查代理是否已存在
				exists, err := ps.ProxyExists(ip, port)
				if err != nil {
					log.Printf("检查代理存在性失败: %v\n", err)
					return
				}

				// 检查代理位置信息是否为空，如果为空使用纯真ip数据库进行补全
				if proxyBase.Country == "" || proxyBase.Province == "" || proxyBase.City == "" {
					locator, err := iploc.Open("data/czutf8.dat")
					if err != nil {
						log.Fatal("出现异常，czutf8.dat 不存在")
						panic(err)
					}
					detail := locator.Find(ip)
					proxyBase.Country = detail.Country
					proxyBase.Province = detail.Province
					proxyBase.City = detail.City
				}

				if exists {
					// 更新代理信息
					err = ps.UpdateProxy(ip, port, proxyBase.Country, proxyBase.Province, proxyBase.City)
					if err != nil {
						log.Printf("更新代理 %s 失败: %v\n", res.ProxyAddr, err)
					} else {
						log.Printf("更新代理: %s\n", res.ProxyAddr)
					}
				} else {
					// 插入新代理
					err = ps.InsertProxy(ip, port, "http", proxyBase.Country, proxyBase.Province, proxyBase.City)
					if err != nil {
						log.Printf("存储代理 %s 失败: %v\n", res.ProxyAddr, err)
					} else {
						log.Printf("存储可用代理: %s\n", res.ProxyAddr)
					}
				}
			} else {
				log.Printf("代理 %s 不可用: %v\n", res.ProxyAddr, res.Error)
			}
		}(common.ProxyCheckResult(result))
	}

	// 等待所有并发操作完成
	wg.Wait()
}

// fetchHunterData 发起 HTTP 请求并解析 Hunter API 返回的数据
func fetchHunterData(requestURL string) ([]common.ProxyBase, error) {
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	var hunterResponse common.HunterResponse
	err = json.NewDecoder(resp.Body).Decode(&hunterResponse)
	if err != nil {
		return nil, fmt.Errorf("解析 JSON 数据失败: %w", err)
	}

	return hunterResponse.Data.Arr, nil
}

// fetchFofaData 发起 HTTP 请求并解析 Fofa API 返回的数据
func fetchFofaData(requestURL string) ([]common.FofaProxy, error) {
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	var fofaResponse common.FofaResponse
	err = json.NewDecoder(resp.Body).Decode(&fofaResponse)
	if err != nil {
		return nil, fmt.Errorf("解析 JSON 数据失败: %w", err)
	}

	return common.ExtractProxiesFromFofa(fofaResponse), nil
}

// fetchHunterResponse 发起 HTTP 请求并解析 Hunter API 返回的数据
func fetchHunterResponse(requestURL string) (*common.HunterResponse, error) {
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	var hunterResponse common.HunterResponse
	err = json.NewDecoder(resp.Body).Decode(&hunterResponse)
	if err != nil {
		return nil, fmt.Errorf("解析 JSON 数据失败: %w", err)
	}

	return &hunterResponse, nil
}

// fetchFofaResponse 发起 HTTP 请求并解析 Fofa API 返回的数据
func fetchFofaResponse(requestURL string) (*common.FofaResponse, error) {
	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	var fofaResponse common.FofaResponse
	err = json.NewDecoder(resp.Body).Decode(&fofaResponse)
	if err != nil {
		return nil, fmt.Errorf("解析 JSON 数据失败: %w", err)
	}

	return &fofaResponse, nil
}

// GetProxyList 从指定的 API 获取代理列表并返回格式化的代理 URL 列表
func GetProxyList(apiURL string) ([]string, error) {
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	var proxies []common.ProxyData
	err = json.NewDecoder(resp.Body).Decode(&proxies)
	if err != nil {
		return nil, fmt.Errorf("解析 JSON 数据失败: %v", err)
	}

	var proxyList []string
	for _, proxy := range proxies {
		proxyList = append(proxyList, fmt.Sprintf("http://%s", proxy.Proxy))
	}

	return proxyList, nil
}

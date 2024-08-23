package core

import (
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"proxychain/common"
	"proxychain/database"
	"proxychain/proxyPool"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/proxy"
)

var (
	GlobeProxyList []string
	proxyIndex     = 0
	mu             sync.Mutex // 保护 proxyIndex 的并发访问
	ps_tmp         *database.ProxyStorage
	usageCount     = make(map[string]int) // 记录每个代理的使用次数
)

// loadProxies 从数据库中加载10个代理地址
func loadProxies(ps *database.ProxyStorage) {
	ps_tmp = ps

	var err error
	var proxyList = make([]string, 0)

	// 检查是否只获取中国的代理
	onlyChina := common.GlobalConfig.Config.OnlyChina

	// 按照模式来决定获取代理
	if common.GlobalConfig.Config.ObtainingProxyMode == "random" {
		if onlyChina {
			proxyList, err = ps.GetRandomProxiesFromCountry(10, "中国")
		} else {
			proxyList, err = ps.GetRandomProxies(10)
		}
		if err != nil {
			log.Fatalf("获取代理列表失败: %v", err)
		}
		log.Printf("更新当前代理列表 random %v", proxyList)
	} else if common.GlobalConfig.Config.ObtainingProxyMode == "priority" {
		if onlyChina {
			proxyList, err = ps.GetActiveProxiesByPriorityFromCountry(10, "中国")
		} else {
			proxyList, err = ps.GetActiveProxiesByPriorityLimit(10)
		}
		if err != nil {
			log.Fatalf("获取代理列表失败: %v", err)
		}
		log.Printf("更新当前代理列表 priority %v", proxyList)
	}

	if len(proxyList) == 0 {
		log.Println("数据库中没有可用的代理IP。")
		proxyPool.GetProxyBase(ps)
	}

	GlobeProxyList = proxyList
}

// getNextProxy 返回下一个代理地址
func getNextProxy() string {
	mu.Lock()
	defer mu.Unlock()

	if len(GlobeProxyList) == 0 {
		log.Println("代理列表为空，无法获取下一个代理。")
		proxyPool.GetProxyBase(ps_tmp)
		return ""
	}

	proxy := GlobeProxyList[proxyIndex]
	proxyIndex = (proxyIndex + 1) % len(GlobeProxyList)

	// 增加使用次数
	usageCount[proxy]++

	return proxy
}

// refreshProxyList 刷新代理列表并重置计数
func refreshProxyList() {
	loadProxies(ps_tmp)
	proxyIndex = 0
	usageCount = make(map[string]int) // 重置使用次数计数
}

func createDialer(proxyURL string) (proxy.Dialer, error) {
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	switch parsedURL.Scheme {
	case "socks5":
		return proxy.SOCKS5("tcp", parsedURL.Host, nil, proxy.Direct)
	case "http":
		return &httpProxyDialer{proxyURL: proxyURL}, nil
	default:
		return nil, errors.New("不支持的代理协议: " + parsedURL.Scheme)
	}
}

type httpProxyDialer struct {
	proxyURL string
}

func (d *httpProxyDialer) Dial(network, addr string) (net.Conn, error) {
	proxyURL, err := url.Parse(d.proxyURL)
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial("tcp", proxyURL.Host)
	if err != nil {
		return nil, err
	}

	req := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Host: addr},
		Header: make(http.Header),
		Host:   addr,
	}

	err = req.Write(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, errors.New("HTTP 代理连接失败: " + resp.Status)
	}

	return conn, nil
}

func HandleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	proxyURL := getNextProxy()
	if proxyURL == "" {
		log.Printf("[%s] 无法获取代理，连接关闭。\n", clientAddr)
		return
	}

	// 解析IP和端口，准备在成功时增加优先级
	ip, port := extractIPAndPort(proxyURL)

	dialer, err := createDialer(proxyURL)
	if err != nil {
		log.Printf("[%s] 使用代理: %s, 创建拨号器失败: %v\n", clientAddr, proxyURL, err)
		decreaseProxyPriority(ip, port)
		tryNextProxy(clientConn)
		return
	}

	clientReader := bufio.NewReader(clientConn)
	request, err := http.ReadRequest(clientReader)
	if err != nil {
		log.Printf("[%s] 使用代理: %s, 读取HTTP请求失败: %v\n", clientAddr, proxyURL, err)
		decreaseProxyPriority(ip, port)
		tryNextProxy(clientConn)
		return
	}

	host := request.Host
	if !strings.Contains(host, ":") {
		if request.URL.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	// 添加 Accept-Encoding 头以支持 gzip 压缩
	request.Header.Set("Accept-Encoding", "gzip")

	log.Printf("[%s] 使用代理: %s, 原地址: %s -> 目标地址: %s",
		clientAddr, proxyURL, clientAddr, host)

	if request.Method == http.MethodConnect {
		handleHTTPS(clientConn, request, dialer, ip, port)
	} else {
		handleHTTP(clientConn, request, dialer, ip, port)
	}
}

func handleHTTPS(clientConn net.Conn, request *http.Request, dialer proxy.Dialer, ip string, port int) {
	host := request.Host

	serverConn, err := dialer.Dial("tcp", host)
	if err != nil {
		log.Printf("连接到服务器失败: %v\n", err)
		decreaseProxyPriority(ip, port)
		tryNextProxy(clientConn)
		return
	}
	defer serverConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	go io.Copy(serverConn, clientConn)
	io.Copy(clientConn, serverConn)

	// 增加成功代理的优先级
	increaseProxyPriority(ip, port)
}

func handleHTTP(clientConn net.Conn, request *http.Request, dialer proxy.Dialer, ip string, port int) {
	host := request.Host
	if !strings.Contains(host, ":") {
		if request.URL.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	serverConn, err := dialer.Dial("tcp", host)
	if err != nil {
		log.Printf("连接到服务器失败: %v\n", err)
		decreaseProxyPriority(ip, port)
		tryNextProxy(clientConn)
		return
	}
	defer serverConn.Close()

	err = request.Write(serverConn)
	if err != nil {
		log.Printf("写入请求到服务器失败: %v\n", err)
		decreaseProxyPriority(ip, port)
		tryNextProxy(clientConn)
		return
	}

	// 读取服务器响应
	serverReader := bufio.NewReader(serverConn)
	response, err := http.ReadResponse(serverReader, request)
	if err != nil {
		log.Printf("读取服务器响应失败: %v\n", err)
		decreaseProxyPriority(ip, port)
		tryNextProxy(clientConn)
		return
	}
	defer response.Body.Close()

	// 解压缩服务器响应
	var reader io.Reader
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		if err != nil {
			log.Printf("创建gzip解压缩器失败: %v\n", err)
			decreaseProxyPriority(ip, port)
			tryNextProxy(clientConn)
			return
		}
		defer reader.(*gzip.Reader).Close()
	default:
		reader = response.Body
	}

	// 将响应写回客户端
	err = response.Write(clientConn)
	if err != nil {
		log.Printf("写入响应到客户端失败: %v\n", err)
		decreaseProxyPriority(ip, port)
		tryNextProxy(clientConn)
		return
	}
	io.Copy(clientConn, reader)

	// 增加成功代理的优先级
	increaseProxyPriority(ip, port)
}

// tryNextProxy 更换代理并重试连接
func tryNextProxy(clientConn net.Conn) {
	clientAddr := clientConn.RemoteAddr().String()
	proxyURL := getNextProxy()
	if proxyURL == "" {
		log.Printf("[%s] 无法获取新的代理，连接关闭。\n", clientAddr)
		return
	}

	// 解析IP和端口，准备在成功时增加优先级
	ip, port := extractIPAndPort(proxyURL)

	dialer, err := createDialer(proxyURL)
	if err != nil {
		log.Printf("[%s] 使用新代理: %s, 创建拨号器失败: %v\n", clientAddr, proxyURL, err)
		decreaseProxyPriority(ip, port)
		return
	}

	// 重新尝试连接
	clientReader := bufio.NewReader(clientConn)
	request, err := http.ReadRequest(clientReader)
	if err != nil {
		log.Printf("[%s] 使用新代理: %s, 读取HTTP请求失败: %v\n", clientAddr, proxyURL, err)
		decreaseProxyPriority(ip, port)
		return
	}

	host := request.Host
	if !strings.Contains(host, ":") {
		if request.URL.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	// 添加 Accept-Encoding 头以支持 gzip 压缩
	request.Header.Set("Accept-Encoding", "gzip")

	log.Printf("[%s] 使用新代理: %s, 原地址: %s -> 目标地址: %s",
		clientAddr, proxyURL, clientAddr, host)

	if request.Method == http.MethodConnect {
		handleHTTPS(clientConn, request, dialer, ip, port)
	} else {
		handleHTTP(clientConn, request, dialer, ip, port)
	}
}

// decreaseProxyPriority 调用数据库接口降低代理的优先级
func decreaseProxyPriority(ip string, port int) {
	err := ps_tmp.DecreasePriority(ip, port)
	if err != nil {
		log.Printf("降低代理优先级失败: %v\n", err)
	}
}

// increaseProxyPriority 调用数据库接口增加代理的优先级
func increaseProxyPriority(ip string, port int) {
	err := ps_tmp.IncreasePriority(ip, port)
	if err != nil {
		log.Printf("增加代理优先级失败: %v\n", err)
	}
}

// extractIPAndPort 从代理地址中提取 IP 和端口
func extractIPAndPort(proxyAddr string) (string, int) {
	parsedURL, err := url.Parse(proxyAddr)
	if err != nil {
		log.Printf("解析代理地址失败: %v", err)
		return "", 0
	}

	host := parsedURL.Hostname()
	portStr := parsedURL.Port()

	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Printf("转换端口失败: %v", err)
		return "", 0
	}

	return host, port
}

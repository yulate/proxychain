package proxyPool

import (
	"bufio"
	"errors"
	"fmt"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ProxyCheckResult 结构体保存代理检测的结果
type ProxyCheckResult struct {
	ProxyAddr  string
	Success    bool
	SuccessURL string
	Error      error
}

// CheckProxy 并发检测代理的可用性，并返回结构化的检测结果
func CheckProxy(proxies []string, targetURLs []string) []ProxyCheckResult {
	var wg sync.WaitGroup
	results := make(chan ProxyCheckResult, len(proxies))

	for _, proxyAddr := range proxies {
		wg.Add(1)
		go func(proxyAddr string) {
			defer wg.Done()
			result := checkProxy(proxyAddr, targetURLs)
			results <- result
		}(proxyAddr)
	}

	// 等待所有 goroutines 完成
	wg.Wait()
	close(results)

	// 收集结果
	var finalResults []ProxyCheckResult
	for result := range results {
		finalResults = append(finalResults, result)
		fmt.Println(formatResult(result)) // 输出结果
	}

	return finalResults
}

// checkProxy 根据传入的代理地址检测其可用性，并返回结构化的结果
func checkProxy(proxyAddr string, targetURLs []string) ProxyCheckResult {
	parsedURL, err := url.Parse(proxyAddr)
	if err != nil {
		return ProxyCheckResult{ProxyAddr: proxyAddr, Success: false, Error: fmt.Errorf("解析代理URL失败: %w", err)}
	}

	dialer, err := createDialer(parsedURL)
	if err != nil {
		return ProxyCheckResult{ProxyAddr: proxyAddr, Success: false, Error: err}
	}

	for _, targetURL := range targetURLs {
		if err := tryRequest(targetURL, dialer); err == nil {
			return ProxyCheckResult{ProxyAddr: proxyAddr, Success: true, SuccessURL: targetURL}
		}
	}

	return ProxyCheckResult{ProxyAddr: proxyAddr, Success: false, Error: errors.New("所有目标请求均失败")}
}

// createDialer 根据代理协议创建对应的拨号器
func createDialer(parsedURL *url.URL) (proxy.Dialer, error) {
	switch parsedURL.Scheme {
	case "socks5":
		return proxy.SOCKS5("tcp", parsedURL.Host, nil, proxy.Direct)
	case "http":
		return &httpProxyDialer{proxyURL: parsedURL.String()}, nil
	default:
		return nil, errors.New("不支持的代理协议: " + parsedURL.Scheme)
	}
}

// tryRequest 尝试通过代理请求目标 URL
func tryRequest(targetURL string, dialer proxy.Dialer) error {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: dialer.Dial,
		},
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(targetURL)
	if err != nil {
		return fmt.Errorf("请求 %s 失败: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("目标 %s 响应异常: 状态码 %d", targetURL, resp.StatusCode)
	}

	return nil
}

// httpProxyDialer 自定义的HTTP代理Dialer
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

// formatResult 格式化结果输出
func formatResult(result ProxyCheckResult) string {
	if result.Success {
		return fmt.Sprintf("代理 %s 可用，成功访问: %s", result.ProxyAddr, result.SuccessURL)
	}
	return fmt.Sprintf("代理 %s 不可用: %v", result.ProxyAddr, result.Error)
}

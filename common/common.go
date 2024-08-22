package common

import (
	"log"
	"net/url"
	"strconv"
)

type ProxyCheckResult struct {
	ProxyAddr  string
	Success    bool
	SuccessURL string
	Error      error
}

func ExtractIPAndPort(proxyAddr string) (string, int, error) {
	parsedURL, err := url.Parse(proxyAddr)
	if err != nil {
		log.Printf("解析代理地址失败: %v", err)
		return "", 0, err
	}

	host := parsedURL.Hostname()
	portStr := parsedURL.Port()

	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Printf("转换端口失败: %v", err)
		return "", 0, err
	}

	return host, port, nil
}

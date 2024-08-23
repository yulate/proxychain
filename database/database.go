package database

import (
	"database/sql"
	"fmt"
	"proxychain/common"
	"time"
)

// SQL 语句常量定义
var (
	createTableQuery = `
		CREATE TABLE IF NOT EXISTS proxies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ip TEXT NOT NULL,
			port INTEGER NOT NULL,
			protocol TEXT NOT NULL,
			country TEXT,
			province TEXT,
			city TEXT,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			priority INTEGER NOT NULL DEFAULT 100,
			last_checked DATETIME
		);
	`
	insertProxyQuery = `
		INSERT INTO proxies (ip, port, protocol, country, province, city, priority, last_checked)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`
	updatePriorityQuery = `
		UPDATE proxies
		SET priority = priority - ?, last_checked = ?
		WHERE ip = ? AND port = ?;
	`
	increasePriorityQuery = `
		UPDATE proxies
		SET priority = priority + ?, last_checked = ?
		WHERE ip = ? AND port = ?;
	`
	getActiveProxiesQuery = `
		SELECT ip, port, protocol, country, province, city
		FROM proxies
		WHERE is_active = 1
		ORDER BY priority DESC;
	`
	deleteLowPriorityProxiesQuery = `
		DELETE FROM proxies
		WHERE priority < 0;
	`
	getProxyCountQuery = `
		SELECT COUNT(*) FROM proxies;
	`
	proxyExistsQuery = `
		SELECT COUNT(*)
		FROM proxies
		WHERE ip = ? AND port = ?;
	`
	updateProxyQuery = `
		UPDATE proxies
		SET country = ?, province = ?, city = ?, last_checked = ?
		WHERE ip = ? AND port = ?;
	`

	createHighPriorityProxyTableQuery = `
		CREATE TABLE IF NOT EXISTS high_proiority_proxies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ip TEXT NOT NULL,
			port INTEGER NOT NULL,
			protocol TEXT NOT NULL,
			country TEXT,
			province TEXT,
			city TEXT,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			priority INTEGER NOT NULL DEFAULT 100,
			last_checked DATETIME
		);
	`
)

// NewProxyStorage 初始化数据库并创建表
func NewProxyStorage(dbPath string) (*ProxyStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if _, err = db.Exec(createTableQuery); err != nil {
		return nil, err
	}

	if _, err = db.Exec(createHighPriorityProxyTableQuery); err != nil {
		return nil, err
	}

	return &ProxyStorage{db: db}, nil
}

// InsertProxy 插入新的代理信息，并设置默认优先级
func (ps *ProxyStorage) InsertProxy(ip string, port int, protocol, country, province, city string) error {
	_, err := ps.db.Exec(insertProxyQuery, ip, port, protocol, country, province, city, 100, time.Now())
	return err
}

// DecreasePriority 降低代理的优先级
func (ps *ProxyStorage) DecreasePriority(ip string, port int) error {
	_, err := ps.db.Exec(updatePriorityQuery, common.GlobalConfig.Config.PriorityDownNum, time.Now(), ip, port)
	return err
}

// GetActiveProxiesByPriority 按优先级获取所有可用的代理
func (ps *ProxyStorage) GetActiveProxiesByPriority() ([]common.ProxyBase, error) {
	rows, err := ps.db.Query(getActiveProxiesQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proxies []common.ProxyBase
	for rows.Next() {
		var proxy common.ProxyBase
		err := rows.Scan(&proxy.IP, &proxy.Port, &proxy.Protocol, &proxy.Country, &proxy.Province, &proxy.City)
		if err != nil {
			return nil, err
		}
		proxy.URL = fmt.Sprintf("%s://%s:%d", proxy.Protocol, proxy.IP, proxy.Port)
		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

// DeleteLowPriorityProxies 删除优先级低于0的代理
func (ps *ProxyStorage) DeleteLowPriorityProxies() error {
	_, err := ps.db.Exec(deleteLowPriorityProxiesQuery)
	return err
}

// GetRandomProxies 随机从数据库中取出指定数量的代理
func (ps *ProxyStorage) GetRandomProxies(limit int) ([]string, error) {
	query := `
		SELECT ip, port, protocol
		FROM proxies
		WHERE is_active = 1
		ORDER BY RANDOM()
		LIMIT ?;
	`
	rows, err := ps.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proxies []string
	for rows.Next() {
		var ip, protocol string
		var port int
		err := rows.Scan(&ip, &port, &protocol)
		if err != nil {
			return nil, err
		}
		fullURL := fmt.Sprintf("%s://%s:%d", protocol, ip, port)
		proxies = append(proxies, fullURL)
	}

	return proxies, nil
}

// GetActiveProxiesByPriorityLimit 获取按优先级排序的代理，最多获取指定数量
func (ps *ProxyStorage) GetActiveProxiesByPriorityLimit(limit int) ([]string, error) {
	query := `
		SELECT ip, port, protocol
		FROM proxies
		WHERE is_active = 1
		ORDER BY priority DESC
		LIMIT ?;
	`
	rows, err := ps.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proxies []string
	for rows.Next() {
		var ip, protocol string
		var port int
		err := rows.Scan(&ip, &port, &protocol)
		if err != nil {
			return nil, err
		}
		fullURL := fmt.Sprintf("%s://%s:%d", protocol, ip, port)
		proxies = append(proxies, fullURL)
	}

	return proxies, nil
}

// IncreasePriority 增加代理的优先级
func (ps *ProxyStorage) IncreasePriority(ip string, port int) error {
	_, err := ps.db.Exec(increasePriorityQuery, common.GlobalConfig.Config.PriorityUpNum, time.Now(), ip, port)
	return err
}

// GetProxyCount 获取数据库中代理的数量
func (ps *ProxyStorage) GetProxyCount() (int, error) {
	var count int
	err := ps.db.QueryRow(getProxyCountQuery).Scan(&count)
	return count, err
}

// ProxyExists 检查数据库中是否存在指定的代理
func (ps *ProxyStorage) ProxyExists(ip string, port int) (bool, error) {
	var count int
	err := ps.db.QueryRow(proxyExistsQuery, ip, port).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateProxy 更新现有代理的信息
func (ps *ProxyStorage) UpdateProxy(ip string, port int, country, province, city string) error {
	_, err := ps.db.Exec(updateProxyQuery, country, province, city, time.Now(), ip, port)
	return err
}

// GetCountryStatistics 获取数据库中 country 为 "中国" 和其他国家的代理数量
func (ps *ProxyStorage) GetCountryStatistics() (int, int, error) {
	query := `
		SELECT 
			SUM(CASE WHEN country = '中国' THEN 1 ELSE 0 END) AS china_count,
			SUM(CASE WHEN country != '中国' OR country IS NULL THEN 1 ELSE 0 END) AS non_china_count
		FROM proxies;
	`

	var chinaCount int
	var nonChinaCount int
	err := ps.db.QueryRow(query).Scan(&chinaCount, &nonChinaCount)
	if err != nil {
		return 0, 0, err
	}

	return chinaCount, nonChinaCount, nil
}

// GetRandomProxiesFromCountry 随机获取指定国家的代理
func (ps *ProxyStorage) GetRandomProxiesFromCountry(limit int, country string) ([]string, error) {
	query := `
		SELECT ip, port, protocol
		FROM proxies
		WHERE is_active = 1 AND country = ?
		ORDER BY RANDOM()
		LIMIT ?;
	`
	rows, err := ps.db.Query(query, country, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proxies []string
	for rows.Next() {
		var ip, protocol string
		var port int
		err := rows.Scan(&ip, &port, &protocol)
		if err != nil {
			return nil, err
		}
		fullURL := fmt.Sprintf("%s://%s:%d", protocol, ip, port)
		proxies = append(proxies, fullURL)
	}

	return proxies, nil
}

// GetActiveProxiesByPriorityFromCountry 获取指定国家的按优先级排序的代理
func (ps *ProxyStorage) GetActiveProxiesByPriorityFromCountry(limit int, country string) ([]string, error) {
	query := `
		SELECT ip, port, protocol
		FROM proxies
		WHERE is_active = 1 AND country = ?
		ORDER BY priority DESC
		LIMIT ?;
	`
	rows, err := ps.db.Query(query, country, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proxies []string
	for rows.Next() {
		var ip, protocol string
		var port int
		err := rows.Scan(&ip, &port, &protocol)
		if err != nil {
			return nil, err
		}
		fullURL := fmt.Sprintf("%s://%s:%d", protocol, ip, port)
		proxies = append(proxies, fullURL)
	}

	return proxies, nil
}

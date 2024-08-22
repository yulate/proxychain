package common

// ProxyData 表示 JSON 数据中的每个对象
type ProxyData struct {
	Anonymous  string `json:"anonymous"`
	CheckCount int    `json:"check_count"`
	FailCount  int    `json:"fail_count"`
	Https      bool   `json:"https"`
	LastStatus bool   `json:"last_status"`
	LastTime   string `json:"last_time"`
	Proxy      string `json:"proxy"`
	Region     string `json:"region"`
	Source     string `json:"source"`
}

// HunterResponse 是从 Hunter API 返回的 JSON 数据结构
type HunterResponse struct {
	Code int `json:"code"`
	Data struct {
		Total int         `json:"total"`
		Arr   []ProxyBase `json:"arr"`
	} `json:"data"`
}

// ProxyBase 表示每个代理的基本信息
type ProxyBase struct {
	URL       string `json:"url"`
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	Country   string `json:"country"`
	Province  string `json:"province"`
	City      string `json:"city"`
	UpdatedAt string `json:"updated_at"`
}

// FofaResponse 是从 Fofa API 返回的 JSON 数据结构
type FofaResponse struct {
	Error           bool       `json:"error"`
	ConsumedFPoint  int        `json:"consumed_fpoint"`
	RequiredFPoints int        `json:"required_fpoints"`
	Size            int        `json:"size"`
	Tip             string     `json:"tip"`
	Page            int        `json:"page"`
	Mode            string     `json:"mode"`
	Query           string     `json:"query"`
	Results         [][]string `json:"results"`
}

// FofaProxy 表示每个 Fofa 返回的代理的信息
type FofaProxy struct {
	FullAddress string `json:"full_address"`
	IP          string `json:"ip"`
	Port        string `json:"port"`
}

// ExtractProxiesFromFofa 提取并返回解析后的代理列表
func ExtractProxiesFromFofa(response FofaResponse) []FofaProxy {
	var proxies []FofaProxy

	for _, result := range response.Results {
		if len(result) >= 3 {
			proxies = append(proxies, FofaProxy{
				FullAddress: result[0],
				IP:          result[1],
				Port:        result[2],
			})
		}
	}

	return proxies
}

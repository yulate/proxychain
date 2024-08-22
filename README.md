# proxychain
从fofa和hunter获取开放的代理池，提取其中的可用ip做第二次筛选，按照置信度进行提取聚合成一个单一的http代理供用户使用，该工具可以做到每次请求都使用不同的ip地址。


置信度计算方式：

- 加分项：
  - 使用该代理成功请求指定目标成功
  - 在定时检测中检测成功
- 减分项
  - 使用该代理成功请求指定目标失败
  - 在定时检测中检测失败

请求成功率优化方法：
 - 按照置信度提取代理进行使用
 - 当第一次请求失败使用新的代理重放该次请求
 - todo

## Usage

编译或使用releases中的二进制包，在二进制程序的同级目录需要存在config.yaml，该文件为proxychain的配置文件。具体解析如下：
```yaml
server:
  # 代理服务器启动的地址与端口配置
  host: "127.0.0.1"
  port: "33445"

database:
  # 数据库类型暂时不可改变
  type: "sqlite"
  path: "proxychain.db"

hunter:
  # 鹰图key 只需要积分即可
  apiKet: ""
fofa:
  # fofa key需要为高级会员，或有足够的积分
  apiKey: ""

config:
  # 精确地区代理，请求中国的地址使用中国的代理，如果是国外的地址则使用国外的代理
  preciseArea: false
  # 获取代理的模式，现有可选模式：priority（优先级）、random（随机）
  obtainingProxyMode: "random"
  # 数据库中最少代理数量的阈值
  miniProxyCount: 80
  # 定时任务执行间隔，单位秒
  taskTime: 60
  # 可信度每次减少的值
  priorityDownNum: 10
  # 可信度每次增加的值
  priorityUpNum: 2
```

编辑好配置文件即可启动
```
chmod 777 proxychain
./proxychain
```
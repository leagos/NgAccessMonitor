# version 1.0

## 功能简介
* 实时监控nginx access_log ,通过百度地图api获取ip位置信息，如果是国外ip访问，则推送钉钉报警
* 经过测试百度地图api不能定位国外ip，目前默认不能定位是国内的ip即国外ip

## 配置说明
### 配置文件：config/config
### 配置项
### log_format
* nginx log_format配置

* 举例：
$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent $request_time $upstream_response_time "$http_referer" "$http_user_agent" $http_x_forwarded_for
#### http_x_forwarded_for_index
* http_x_forwarded_for的位置，以上述配置为例，则该配置值为11
#### request_index
* request的位置，以上述配置为例，则该配置值为4
#### accessLogPath
* nginx access_log 路径

#### ding_robot_url
* 钉钉群机器人url，关键词：监控报警

#### baidu_map_ak
* 百度地图AK

## 运行说明
* 命令： nohup ./monitor [消费者数量] > run.log &  





package tools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var Config = make(map[string]string)
var Logger log.Logger
var logPattern string

func InitConfig(path string) {
	f, err := os.Open(path)
	defer f.Close()
	if err != nil {
		panic(err)
	}

	r := bufio.NewReader(f)
	for {
		b, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		s := strings.TrimSpace(string(b))
		index := strings.Index(s, "=")
		if index < 0 {
			continue
		}
		key := strings.TrimSpace(s[:index])
		if len(key) == 0 {
			continue
		}
		value := strings.TrimSpace(s[index+1:])
		if len(value) == 0 {
			panic(key + "未配置")
		}
		Config[key] = value
	}
}

func PreCheck() {
	//检查钉钉是否配置
	if ding_robot_url, ok := Config["ding_robot_url"]; !ok || len(ding_robot_url) == 0 {
		log.Panic("钉钉机器人未配置")
	}
	//百度ak
	baiduMapAk, ok := Config["baidu_map_ak"]
	if !ok || len(baiduMapAk) == 0 {
		panic("百度地图AK未配置")
	}

	//日志格式
	if logFormat, ok := Config["log_format"]; !ok || len(logFormat) == 0 {
		panic("日志格式未配置")
	}

}

var LogPatternMap = map[string]string{
	"$time_local":             `[^\[\]]+`,
	"$server_addr":            `\d+\.\d+\.\d+\.\d+`,
	"$remote_addr":            `\d+\.\d+\.\d+\.\d+`,
	"$request_time":           `\d*\.\d*|\-`,
	"$http_host":              `[^\s]*`,
	"$http_x_forwarded_for":   `\d+\.\d+\.\d+\.\d+\,\s\d+\.\d+\.\d+\.\d+|\d+\.\d+\.\d+\.\d+`,
	"$request":                `(?:[^"]|)+|-`,
	"$http_user_agent":        `(?:[^"]|)+|-`,
	"$remote_user":            `(?:[^"]|)+|-`,
	"$http_referer":           `(?:[^"]|)+|-`,
	"$status":                 `\d{3}`,
	"$body_bytes_sent":        `\d+|-`,
	"$upstream_response_time": `\d*\.\d*|\-`,
}

//生成正则表达式
func GetPattern(logFormat string) (p string) {
	//fmt.Println(logFormat)
	//替换" 和 []
	logFormat = strings.Replace(logFormat, "\"", "\\\"", -1)
	logFormat = strings.Replace(logFormat, "[", "\\[", -1)
	logFormat = strings.Replace(logFormat, "]", "\\]", -1)
	//fmt.Println(logFormat)

	for k, v := range LogPatternMap {
		logFormat = strings.Replace(logFormat, k, "("+v+")", -1)
	}
	return logFormat
}

//解析log
func parseLog(line, regPattern string) (status bool, msg string, rst []string, ipIdx int) {
	// r := regexp.MustCompile(`(\[(\d{2})\/\w{3}\/\d{4}:\d{2}:\d{2}:\d{2}) \+\d{4}\] (\"(?:[^"]|\")+|-\") (\d{3}) (\d*\.\d*|\-)  (\d*\.\d*|\-) \"-\" (\"(?:[^"]|\")+|-\") (\d+\.\d+\.\d+\.\d+)`)
	// r := regexp.MustCompile(`([\d\.]+)\s+([^\[]+)\s+([^\[]+)\s+\[([^\]]+)\]\s+\"([^"]+)\"\s+(\d{3})\s+(\d+)\s+\"([^"]+)\"\s+\"([^"]+)\"\s+`)
	//r := regexp.MustCompile(`([\d\.]+)\s+([^\[]+)\s+([^\[]+)\s+\[([^\]]+)\]\s+\"([^"]+)\"\s+(\d{3})\s+(\d+)\s+\"([^"]+)\"\s+\"([^"]+)\"\s+`)
	status = false
	msg = ""

	r := regexp.MustCompile(regPattern)
	ret := r.FindStringSubmatch(line)
	if position, ok := Config["http_x_forwarded_for_index"]; ok && len(position) > 0 {
		idx, err := strconv.Atoi(position)
		if err == nil {
			if len(ret) > idx {
				//ip := ret[idx]
				//go GetIPLocation(ret, idx)
				status = true
				rst = ret
				ipIdx = idx
			} else {
				msg = "wrong log Format"
				//fmt.Println("wrong log Format")
				//fmt.Println(line)
			}
		} else {
			msg = "get http_x_forwarded_for_index failed"
			//fmt.Println("get http_x_forwarded_for_index failed")
			//fmt.Println(err)
			//fmt.Println(ret)
		}
	}
	return
}

//status false获取失败 location ！= "CN" 则是国外ip
func GetIPLocation(ret []string, ipIdx int) (status bool, location, msg string) {
	status = false
	location = "CN"
	msg = ""
	client := &http.Client{Timeout: 5 * time.Second}
	baiduMapAk, ok := Config["baidu_map_ak"]
	if !ok || len(baiduMapAk) == 0 {
		panic("百度地图AK未配置")
	}
	url := "http://api.map.baidu.com/location/ip?ak=" + baiduMapAk + "&ip=" + ret[ipIdx]
	resp, err := client.Get(url)
	if err != nil {
		panic(err.Error())
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// handle error
		panic(err.Error())
	}
	var d map[string]interface{}
	// 将字符串反解析为字典
	json.Unmarshal(body, &d)
	fmt.Println(d)
	sep := "|"
	if address, ok := d["address"]; ok {
		arr := strings.Split(address.(string), sep)
		if len(arr) > 0 && arr[0] != "CN" {
			fmt.Printf("ip not china,ip location:%v\n", arr[0])
			text := "## 国外IP访问，请检查\n " +
				"> 1. IP:" + ret[ipIdx] + "\n"
			if requestIdx, ok := Config["request_index"]; ok && len(requestIdx) > 0 {
				reqIdxInt, err := strconv.Atoi(requestIdx)
				if err == nil {
					if len(ret) > reqIdxInt {
						//ip := ret[idx]
						text += "> 2. Dest:" + ret[reqIdxInt] + "\n"
					}
				}
			}
			status = true
			location = "F"
			msg = text

			//postData := map[string]interface{}{
			//	"msgtype": "markdown",
			//	"markdown": map[string]string{
			//		"title": "监控报警",
			//		"text":  text,
			//	},
			//}
			//sendDingMsg(postData)
		} else {
			status = true
			location = "CN"
			msg = ""
			//fmt.Printf("ip:%v, location:%v\n", ret[ipIdx], arr[0])
		}
	} else {
		if msg, ok := d["message"]; ok {
			if strings.Contains(msg.(string), "Internal Service Error") {
				text := "## 国外IP访问，请检查\n " +
					"> 1. IP:" + ret[ipIdx] + "\n"
				if requestIdx, ok := Config["request_index"]; ok {
					reqIdxInt, err := strconv.Atoi(requestIdx)
					if err == nil {
						if len(ret) > reqIdxInt {
							//ip := ret[idx]
							text += "> 2. Dest:" + ret[reqIdxInt] + "\n"
						}
					}
				}
				status = true
				location = "F"
				msg = text
				//postData := map[string]interface{}{
				//	"msgtype": "markdown",
				//	"markdown": map[string]string{
				//		"title": "监控报警",
				//		"text":  text,
				//	},
				//}
				//sendDingMsg(postData)
			}
		} else {
			status = false
			location = "unknown"
			msg = "获取" + ret[ipIdx] + "区域失败:" + d["message"].(string)
			return false, "CN", "获取" + ret[ipIdx] + "区域失败" + d["message"].(string)
		}
	}
	return status, location, msg
}

func sendDingMsg(content string) {

	postData := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": "监控报警",
			"text":  content,
		},
	}

	bytesData, err := json.Marshal(postData)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	reader := bytes.NewReader(bytesData)

	//if url, ok := Config["ding_robot_url"]; ok {
	request, err := http.NewRequest("POST", Config["ding_robot_url"], reader)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	client := http.Client{}
	client.Do(request)
	//} else {
	//	panic("钉钉机器人未配置")
	//}
}

/*
   判断文件或文件夹是否存在
   如果返回的错误为nil,说明文件或文件夹存在
   如果返回的错误类型使用os.IsNotExist()判断为true,说明文件或文件夹不存在
   如果返回的错误为其它类型,则不确定是否在存在
*/
func PathExists(path string) (bool, error) {

	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

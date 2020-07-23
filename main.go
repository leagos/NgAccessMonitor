package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nxadm/tail"
)

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
var config = make(map[string]string)

func InitConfig(path string) {
	//config := make(map[string]string)

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
			continue
		}
		config[key] = value
	}
}

func getPattern(logFormat string) (p string) {
	fmt.Println(logFormat)
	logFormat = strings.Replace(logFormat, "\"", "\\\"", -1)
	logFormat = strings.Replace(logFormat, "[", "\\[", -1)
	logFormat = strings.Replace(logFormat, "]", "\\]", -1)
	fmt.Println(logFormat)

	for k, v := range LogPatternMap {
		logFormat = strings.Replace(logFormat, k, "("+v+")", -1)
	}
	return logFormat
}

func parseLog(line, regPattern string) {
	// r := regexp.MustCompile(`(\[(\d{2})\/\w{3}\/\d{4}:\d{2}:\d{2}:\d{2}) \+\d{4}\] (\"(?:[^"]|\")+|-\") (\d{3}) (\d*\.\d*|\-)  (\d*\.\d*|\-) \"-\" (\"(?:[^"]|\")+|-\") (\d+\.\d+\.\d+\.\d+)`)
	// r := regexp.MustCompile(`([\d\.]+)\s+([^\[]+)\s+([^\[]+)\s+\[([^\]]+)\]\s+\"([^"]+)\"\s+(\d{3})\s+(\d+)\s+\"([^"]+)\"\s+\"([^"]+)\"\s+`)
	//r := regexp.MustCompile(`([\d\.]+)\s+([^\[]+)\s+([^\[]+)\s+\[([^\]]+)\]\s+\"([^"]+)\"\s+(\d{3})\s+(\d+)\s+\"([^"]+)\"\s+\"([^"]+)\"\s+`)
	r := regexp.MustCompile(regPattern)
	//fmt.Println(line)
	//fmt.Println(regPattern)
	ret := r.FindStringSubmatch(line)
	if position, ok := config["http_x_forwarded_for_index"]; ok && len(position) > 0 {
		idx, err := strconv.Atoi(position)
		if err == nil {
			if len(ret) > idx {
				//ip := ret[idx]
				go getIPLocation(ret, idx)
			} else {

				fmt.Println("wrong log Format")
			}
		} else {
			fmt.Println(err)
			fmt.Println("get http_x_forwarded_for_index failed")
		}

	}

	// for _, v := range ret {
	// fmt.Println(v)
	// }
}

func getIPLocation(ret []string, ipIdx int) {
	client := &http.Client{Timeout: 5 * time.Second}
	baiduMapAk, ok := config["baidu_map_ak"]
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
			if requestIdx, ok := config["request_index"]; ok && len(requestIdx) > 0 {
				reqIdxInt, err := strconv.Atoi(requestIdx)
				if err == nil {
					if len(ret) > reqIdxInt {
						//ip := ret[idx]
						text += "> 2. Dest:" + ret[reqIdxInt] + "\n"
					}
				}
			}
			postData := map[string]interface{}{
				"msgtype": "markdown",
				"markdown": map[string]string{
					"title": "监控报警",
					"text":  text,
				},
			}
			sendDingMsg(postData)
		} else {
			fmt.Printf("ip:%v, location:%v\n", ret[ipIdx], arr[0])
		}
	} else {
		if msg, ok := d["message"]; ok {
			if strings.Contains(msg.(string), "Internal Service Error") {
				text := "## 国外IP访问，请检查\n " +
					"> 1. IP:" + ret[ipIdx] + "\n"
				if requestIdx, ok := config["request_index"]; ok {
					reqIdxInt, err := strconv.Atoi(requestIdx)
					if err == nil {
						if len(ret) > reqIdxInt {
							//ip := ret[idx]
							text += "> 2. Dest:" + ret[reqIdxInt] + "\n"
						}
					}
				}
				postData := map[string]interface{}{
					"msgtype": "markdown",
					"markdown": map[string]string{
						"title": "监控报警",
						"text":  text,
					},
				}
				sendDingMsg(postData)
			}
		} else {
			fmt.Println("获取" + ret[ipIdx] + "区域失败")
			fmt.Println(d)
		}
	}

}

func sendDingMsg(content map[string]interface{}) {
	bytesData, err := json.Marshal(content)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	reader := bytes.NewReader(bytesData)

	if url, ok := config["ding_robot_url"]; ok {
		request, err := http.NewRequest("POST", url, reader)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		request.Header.Set("Content-Type", "application/json;charset=UTF-8")
		client := http.Client{}
		client.Do(request)
		//resp, err := client.Do(request)
		//if err != nil {
		//	fmt.Println(err.Error())
		//	return
		//}
		//respBytes, err := ioutil.ReadAll(resp.Body)
		//if err != nil {
		//	fmt.Println(err.Error())
		//	return
		//}
		////byte数组直接转成string，优化内存
		//str := (*string)(unsafe.Pointer(&respBytes))
		//fmt.Println(*str)
	} else {
		panic("钉钉机器人未配置")
	}
}

func main() {
	InitConfig("config/config")
	//fmt.Println(config)
	if logFormat, ok := config["log_format"]; ok && len(logFormat) > 0 {
		p := getPattern(logFormat)
		fmt.Println(p)
		t, _ := tail.TailFile(config["accessLogPath"], tail.Config{Follow: true})
		for line := range t.Lines {
			// fmt.Println(line.Text)
			parseLog(line.Text, p)
		}
	} else {
		panic("config file error,no log_format configured")
	}

}

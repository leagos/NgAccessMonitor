package tools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
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

	//fmt.Println("##regPattern##:" + regPattern)
	r := regexp.MustCompile(regPattern)
	ret := r.FindStringSubmatch(line)
	if position, ok := Config["http_x_forwarded_for_index"]; ok && len(position) > 0 {
		idx, err := strconv.Atoi(position)
		if err == nil {
			if len(ret) > idx {
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
	return status, msg, rst, ipIdx
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
		return false, "CN", "获取" + ret[ipIdx] + "区域失败 - error:" + err.Error()
		//panic(err.Error())
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
	//fmt.Println("通过接口获取ip信息：")
	//fmt.Println(d)
	sep := "|"
	if address, ok := d["address"]; ok {
		arr := strings.Split(address.(string), sep)
		if len(arr) > 0 && arr[0] != "CN" {
			//fmt.Printf("ip not china,ip location:%v\n", arr[0])
			status = true
			location = "F"
		} else {
			status = true
			location = "CN"
			msg = ""
		}
	} else {
		if message, ok := d["message"]; ok {
			if strings.Contains(message.(string), "Internal Service Error") {
				//fmt.Println("####Internal Service Error###")
				status = true
				location = "F"
			}
		} else {
			status = false
			location = "unknown"
			msg = "获取" + ret[ipIdx] + "区域失败:" + d["message"].(string)
			return false, "CN", "获取" + ret[ipIdx] + "区域失败" + d["message"].(string)
		}
	}
	//fmt.Println("##GetIPLocation##：" + msg)
	return status, location, msg
}

func sendDingMsg(content string, ip int) {
	//更新时间
	Update(Db, ip)
	PrintLog("###发送钉钉消息###")
	postData := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": "监控报警",
			"text":  content,
		},
	}
	bytesData, err := json.Marshal(postData)
	if err != nil {
		PrintLog(err.Error())
		return
	}
	reader := bytes.NewReader(bytesData)

	//if url, ok := Config["ding_robot_url"]; ok {
	request, err := http.NewRequest("POST", Config["ding_robot_url"], reader)
	if err != nil {
		PrintLog(err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	client := http.Client{}
	_, err = client.Do(request)
	if err != nil {
		PrintLog(err.Error())
	}
	//PrintLog(rsp)
	PrintLog("###发送钉钉消息 完毕###")
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

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

// IPString2Long 把ip字符串转为数值
func IPString2Long(ip string) (int, error) {
	b := net.ParseIP(ip).To4()
	if b == nil {
		return 0, errors.New("invalid ipv4 format")
	}
	return int(b[3]) | int(b[2])<<8 | int(b[1])<<16 | int(b[0])<<24, nil
}

// Long2IPString 把数值转为ip字符串
func Long2IPString(i int) (string, error) {
	if i > math.MaxUint32 {
		return "", errors.New("beyond the scope of ipv4")
	}
	ip := make(net.IP, net.IPv4len)
	ip[0] = byte(i >> 24)
	ip[1] = byte(i >> 16)
	ip[2] = byte(i >> 8)
	ip[3] = byte(i)
	return ip.String(), nil
}

func GetDingMsgText(ret []string, ipIdx int, fineName string) (text string) {

	text = "## 国外IP访问，请检查\n " +
		"> 1. IP:" + ret[ipIdx] + "\n" +
		"> 2. File:" + filepath.Base(fineName) + "\n"

	if requestIdx, ok := Config["request_index"]; ok && len(requestIdx) > 0 {
		reqIdxInt, err := strconv.Atoi(requestIdx)
		if err == nil {
			if len(ret) > reqIdxInt {
				text += "> 3. URL:" + ret[reqIdxInt] + "\n"
			}
		}
	}
	return text
}

func PrintLog(a ...interface{}) {
	fmt.Println(time.Now().Format("###2006-01-02 15:04:05###"))
	fmt.Println(a)
}

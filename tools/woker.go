package tools

import (
	"database/sql"
	"github.com/nxadm/tail"
	"os"
	"strconv"
	"sync"
	"time"
)

//已经被监控的文件
var WatchedFiles = make(map[string]bool)
var Db *sql.DB

type Job struct {
	accessLog string
	fileName  string
}

//任务信道
//var Jobs = make(chan job, 50)

func Producer(accessLogPath string, jobs chan Job) {
	PrintLog("****开启生产者监控***" + accessLogPath)
	//PrintLog(accessLogPath)
	t, err := tail.TailFile(accessLogPath, tail.Config{Follow: true, Location: &tail.SeekInfo{Offset: 0, Whence: 2}}) //从末尾开始
	if err != nil {
		PrintLog(err)
		return
	}
	for line := range t.Lines {
		//PrintLog(strconv.Itoa(time.Now().Second())+"****新任务***"+line.Text)
		job := Job{line.Text, accessLogPath}
		jobs <- job
	}
	PrintLog("##关闭生产者监控##" + accessLogPath)
}
func consumer(wg *sync.WaitGroup, jobs chan Job, i int) {
	for job := range jobs {
		//PrintLog("我是"+ strconv.Itoa(i)+"号工人,收到任务")
		//PrintLog(job)
		status, msg, rst, ipIdx := parseLog(job.accessLog, logPattern)
		if status {
			ipInt, err := IPString2Long(rst[ipIdx])
			ipInfo, err := FindIp(ipInt, Db)
			checkErr(err)
			Update(Db, ipInt)
			//PrintLog("查询ip")
			if ipInfo.location != "" {
				//查询到
				now := time.Now().Unix()
				//5分钟内不重复发送钉钉
				if ipInfo.location != "CN" {
					//PrintLog("国外IP")
					//PrintLog(ipInfo)
					if (now - ipInfo.wTime) > 5*60 {
						message := GetDingMsgText(rst, ipIdx, job.fileName)
						sendDingMsg(message, ipInt)
					}
				}
			} else {
				ok, location, msg := GetIPLocation(rst, ipIdx)
				if ok {
					Insert(Db, ipInt, location)
					if location != "CN" {
						PrintLog("******国外ip******")
						message := GetDingMsgText(rst, ipIdx, job.fileName)
						sendDingMsg(message, ipInt)
					}
				} else {
					PrintLog("##获取ip区域失败##")
					PrintLog(msg)
				}
			}

		} else {
			PrintLog(msg)
		}
	}
	defer wg.Done()
}
func CreateComsumerPool(noOfConsumers int, jobs chan Job) {
	var wg sync.WaitGroup
	for i := 0; i < noOfConsumers; i++ {
		PrintLog("#worker-", i, " started")
		wg.Add(1)
		go consumer(&wg, jobs, i)
	}
	wg.Wait()
}

func StartMointor() {
	InitConfig("config/config")

	logPattern = GetPattern(Config["log_format"])

	fileChan := make(chan string)
	jobs := make(chan Job)

	defer close(fileChan)
	defer close(jobs)

	done := make(chan bool)
	go CreateWatcher(fileChan)

	//初始化数据库
	db, err := Connect()

	checkErr(err)

	Db = db

	go func() {
		var gRWLock *sync.RWMutex
		gRWLock = new(sync.RWMutex)
		for fileName := range fileChan {
			//PrintLog(WatchedFiles)
			//如果文件存在
			gRWLock.RLock()
			if exist, _ := PathExists(fileName); exist {
				//文件没有被监控 记录文件并
				//PrintLog("###已监控文件###")
				//PrintLog(WatchedFiles)
				if _, ok := WatchedFiles[fileName]; !ok {
					//PrintLog("不在监控队列" + fileName)
					WatchedFiles[fileName] = true
					PrintLog("##开启生产者协程##" + fileName)
					go Producer(fileName, jobs)
				}
			} else {
				delete(WatchedFiles, fileName) //删除元素
			}
			gRWLock.RUnlock()
		}
	}()

	noOfWorkers := 10
	if len(os.Args) > 1 {
		number, err := strconv.Atoi(os.Args[1])
		if err != nil {
			panic(err)
		}
		if number > 0 {
			noOfWorkers = number
		}
	}
	CreateComsumerPool(noOfWorkers, jobs)
	<-done
}

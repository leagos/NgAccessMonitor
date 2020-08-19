package tools

import (
	"database/sql"
	"fmt"
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
}

//任务信道
//var Jobs = make(chan job, 50)

func Producer(accessLogPath string, jobs chan Job) {
	fmt.Println("****开启生产者监控***")
	fmt.Println(accessLogPath)
	t, err := tail.TailFile(accessLogPath, tail.Config{Follow: true, Location: &tail.SeekInfo{Offset: 0, Whence: 2}}) //从末尾开始
	if err != nil {
		fmt.Println(err)
		return
	}
	//for {
	//	time.Sleep(5 * time.Second)
	//	fmt.Println("我还在跑" + accessLogPath)
	//}
	for line := range t.Lines {
		fmt.Println("****获取到新行***")
		fmt.Println(line.Text)
		job := Job{line.Text}
		jobs <- job
	}
	fmt.Println("****关闭生产者监控***")
	//close(Jobs)
}
func consumer(wg *sync.WaitGroup, jobs chan Job) {
	for job := range jobs {
		parseLog(job.accessLog, logPattern)
		status, msg, rst, ipIdx := parseLog(job.accessLog, logPattern)
		if status {

			ipInt, err := IPString2Long(rst[ipIdx])
			ipInfo, err := FindIp(ipInt, Db)
			checkErr(err)
			fmt.Println("查询ip")
			if ipInfo.location != "" {
				//查询到
				now := time.Now().Unix()
				//5分钟内不重复发送钉钉
				if ipInfo.location != "CN" {
					fmt.Println("国外IP")
					fmt.Println(ipInfo)
					fmt.Println(now - ipInfo.wTime)
					if (now - ipInfo.wTime) > 5*60 {
						sendDingMsg(msg, ipInt)
					}
				} else {
					fmt.Println("国内访问")
				}
			} else {
				ok, location, msg := GetIPLocation(rst, ipIdx)
				if ok {
					Insert(Db, ipInt, location)
					if location != "CN" {
						sendDingMsg(msg, ipInt)
					} else {
						fmt.Println("国内访问")
					}
				} else {
					fmt.Println("获取ip区域失败")
					fmt.Println(msg)
				}
			}

		} else {
			fmt.Println(msg)
		}
		//time.Sleep(time.Duration(3) * time.Second)
	}
	defer wg.Done()
}
func CreateComsumerPool(noOfConsumers int, jobs chan Job) {
	var wg sync.WaitGroup
	for i := 0; i < noOfConsumers; i++ {
		fmt.Println("#worker-", i, " started")
		wg.Add(1)
		go consumer(&wg, jobs)
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
			fmt.Println("文件变动" + fileName)
			//如果文件存在
			gRWLock.RLock()
			if exist, _ := PathExists(fileName); exist {
				fmt.Println("文件存在")
				//文件没有被监控 记录文件并
				fmt.Println(WatchedFiles)
				if _, ok := WatchedFiles[fileName]; !ok {
					fmt.Println("不在监控队列" + fileName)
					WatchedFiles[fileName] = true
					fmt.Println("开启生产者协程")
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

package tools

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/nxadm/tail"
)

//已经被监控的文件
var WatchedFiles = make(map[string]bool)

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
	fmt.Println("****获取到新行***")
	for line := range t.Lines {
		fmt.Println(line.Text)
		job := Job{line.Text}
		jobs <- job
	}
	fmt.Println("****获取到新行222***")

	//close(Jobs)
}
func consumer(wg *sync.WaitGroup, jobs chan Job) {
	for job := range jobs {
		parseLog(job.accessLog, logPattern)
		time.Sleep(time.Duration(3) * time.Second)
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
	fileChan := make(chan string)
	jobs := make(chan Job)
	var gRWLock *sync.RWMutex
	gRWLock = new(sync.RWMutex)
	defer close(fileChan)
	defer close(jobs)

	done := make(chan bool)
	go CreateWatcher(fileChan)

	for fileName := range fileChan {
		fmt.Println("读取信道")
		//fileEvent := <-fileChan
		fmt.Println(fileName)
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

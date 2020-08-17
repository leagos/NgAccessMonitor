package tools

import (
	"fmt"
	"log"
	"strings"

	"github.com/fsnotify/fsnotify"
)

func CreateWatcher(fileChan chan string) {
	log.Println("开始监控")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				//判断事件发生的类型，如下5种
				// Create 创建
				// Write 写入
				// Remove 删除
				// Rename 重命名
				// Chmod 修改权限
				if event.Op&fsnotify.Create == fsnotify.Create {
					//log.Println("创建文件 : ", event.Name)
					EventProcess(event.Name, fileChan)
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					//log.Println("写入文件 : ", event.Name)
					EventProcess(event.Name, fileChan)
				}
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					//log.Println("删除文件 : ", event.Name)
					EventProcess(event.Name, fileChan)
				}
				if event.Op&fsnotify.Rename == fsnotify.Rename {
					//log.Println("重命名文件 : ", event.Name)
					EventProcess(event.Name, fileChan)
				}
				if event.Op&fsnotify.Chmod == fsnotify.Chmod {
					//log.Println("修改权限 : ", event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()
	log.Println(Config["accessLogPath"])
	err = watcher.Add(Config["accessLogPath"])
	if err != nil {
		log.Fatal(err)
	}
	<-done
}

func EventProcess(eventName string, fileChannel chan string) {
	//只监控.log文件的变化
	if strings.HasSuffix(eventName, `.log`) {
		fmt.Println("log文件出现：" + eventName)
		//文件存在 丢到信道去
		if ok, _ := PathExists(eventName); ok {
			fmt.Println("写入信道" + eventName)
			fileChannel <- eventName
		}
	}
}

package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"github.com/robfig/cron/v3"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type cmdRecord struct {
	EntryID int
	Cmd     string
	CmdSpec string
}

var (
	defaultDir     *string
	cronConfigFile *string
	outputVerbose  *bool
	c              *cron.Cron
	scheduleMap    map[string][2]string
	scheduleEntrys map[string]cmdRecord
	logFD          *os.File
	logFileName    string
)

func setLog(myLogFileName string) {
	if logFileName != myLogFileName {
		logFD.Close()
	}
	logFD, logErr := os.OpenFile(myLogFileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if logErr != nil {
		log.Println("Fail to find", logErr)
		os.Exit(1)
	}
	log.SetOutput(logFD)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	logFileName = myLogFileName
}

func setPid(pidFileName string) {
	pidFile, logErr := os.OpenFile(pidFileName, os.O_CREATE|os.O_RDWR, 0666)
	if logErr != nil {
		log.Println("Fail to find", pidFile)
		os.Exit(1)
	}
	defer pidFile.Close()

	pid := strconv.Itoa(os.Getpid())
	log.Println("pid:", pid)
	_, _ = pidFile.Write([]byte(pid))
}

func main() {

	// 设置log文件名+日期
	setLog("./logs/log-" + time.Now().Format("2006-01-02") + ".log")
	defer logFD.Close()

	cronConfigFile = flag.String("s", "./config/crontab", "use s, 设置cron配置文件")
	defaultDir = flag.String("dd", "/data/www/lizard.weipaitang.com", "use dd, 设置默认工作目录")
	outputVerbose = flag.Bool("vvv", false, "use vvv, 输出更多调试信息")
	flag.Parse()

	log.Println(*cronConfigFile)
	log.Println(*defaultDir)

	scheduleEntrys = make(map[string]cmdRecord)
	scheduleMap = parseCrontabFile(*cronConfigFile)
	c = cron.New(cron.WithSeconds())

	// 设置每天凌晨调整日志名
	_, _ = c.AddFunc("0 0 0 * * *", func() {
		log.Println("零点设置新的日志名")
		setLog("./logs/log-" + time.Now().Format("2006-01-02") + ".log")
	})

	// 引入配置调度
	for myCmdMd5, cmdAttr := range scheduleMap {
		func(myCmdMd5 string, cmdAttr [2]string) {
			log.Println("读取配置item", myCmdMd5, cmdAttr)
			entryId, err := c.AddFunc(cmdAttr[0], func() {
				runShell(cmdAttr[1], myCmdMd5)
			})
			if *outputVerbose {
				log.Println("加入调度情况: ", entryId, err)
			}
			scheduleEntrys[myCmdMd5] = cmdRecord{EntryID: int(entryId), Cmd: cmdAttr[1], CmdSpec: cmdAttr[0]}
		}(myCmdMd5, cmdAttr)

	}

	if *outputVerbose {
		log.Printf("%#v\n\n", scheduleEntrys)
	}

	c.Start()
	log.Println("cron Start")

	go catchSignal()
	//go func() {
	//	for {
	//		time.Sleep(time.Second * 3)
	//		log.Println(scheduleMap)
	//	}
	//}()

	go setPid("./proc/pid")

	select {}
}

func parseCrontabFile(filePath string) map[string][2]string {
	fs, err := os.OpenFile(filePath, os.O_RDONLY, 066)
	if err != nil {
		log.Println("crontab文件打开失败")
		os.Exit(10000)
	}
	defer fs.Close()

	reader := bufio.NewReader(fs)
	scheduleMap := make(map[string][2]string)
	for {
		readLine, err := reader.ReadString('\n')
		if err == io.EOF {
			log.Println("文件读取结束")
			break
		}

		crontabContent := strings.Split(readLine[:len(readLine)-1], "@")
		if len(crontabContent) == 1 {
			continue
		}
		key := strings.Trim(crontabContent[0], " ")
		value := strings.Trim(crontabContent[1], " ")
		scheduleMap[md5V(value)] = [2]string{key, value}
	}

	return scheduleMap
}

func runShell(cmdStr, cmdStrMd5 string) {
	fileMutex := "./mutex/" + cmdStrMd5 + ".lock"
	if IsExist(fileMutex) {
		log.Println("还在运行跳过:【" + cmdStr + "】:【" + fileMutex + "】")
		return
	} else {
		fp, _ := os.Create(fileMutex)
		fp.Close()
	}
	defer func() {
		_ = os.Remove(fileMutex)
	}()

	log.Println("正在运行:【" + cmdStr + "】:【" + fileMutex + "】")
	cmd := exec.Command("/bin/sh", "-c", cmdStr)

	//预设脚本运行的工作目录
	//cmd.Dir = "/data/www/lizard.weipaitang.com"
	cmd.Dir = *defaultDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println(cmdStrMd5 + " - StdoutPipe: " + err.Error())
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Println(cmdStrMd5+" - StderrPipe: ", err.Error())
		return
	}

	if err := cmd.Start(); err != nil {
		log.Println(cmdStrMd5+" - Start: ", err.Error())
		return
	}

	tmpChannel := make(chan int)

	go func() {
		for {
			select {
			case <-tmpChannel:
				break
			default:

			}
			bufReader := bufio.NewReader(stderr)
			bytesErr, err := bufReader.ReadString('\n')
			if err != nil {

				if err == io.EOF {
					return
				}

				// 文件已关闭
				myErr, pErr := err.(*os.PathError)
				if pErr && myErr.Op == "read" {
					return
				}

				log.Printf(cmdStrMd5+" - Read stderr: %#v", err)
				return
			}

			if len(bytesErr) != 0 {
				log.Printf(cmdStrMd5+" - stderr is not nil: %v", bytesErr)
				return
			}

			time.Sleep(time.Millisecond * 500)
		}
	}()

	go func() {

		for {
			select {
			case <-tmpChannel:
				break
			default:

			}
			bufReader := bufio.NewReader(stdout)
			bytes, err := bufReader.ReadString('\n')
			if err != nil {

				if err == io.EOF {
					return
				}

				// 文件已关闭
				myErr, pErr := err.(*os.PathError)
				if pErr && myErr.Op == "read" {
					return
				}

				log.Printf(cmdStrMd5+" - Read stdout: %#v", err)
				return
			}

			log.Printf(cmdStrMd5+" - stdout: %s", bytes)
			time.Sleep(time.Millisecond * 100)
		}

	}()

	if err := cmd.Wait(); err != nil {
		log.Println(cmdStrMd5+" - Wait: ", err.Error())
		return
	}

	tmpChannel <- 1
}

func md5V(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

func IsExist(f string) bool {
	_, err := os.Stat(f)
	return err == nil || os.IsExist(err)
}

func catchSignal() {
	c := make(chan os.Signal)
	//监听指定信号 ctrl+c kill
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				log.Println("退出", s)
				ExitFunc()
			case syscall.SIGUSR1, syscall.SIGUSR2:
				log.Println("重新加载配置", s)
				reload()
			default:
				log.Println("other", s)
			}
		}
	}()
}

func ExitFunc() {
	log.Println("ExitFunc")

	// 删除pid文件
	_ = os.Remove("./proc/pid")

	// 删除锁文件
	filepathNames, err := filepath.Glob("./mutex/*")
	if err != nil {
		log.Fatal(err)
	}
	if len(filepathNames) > 0 {
		log.Println("需删除的mutex锁文件")
		for i := range filepathNames {
			tmpFile := "./proc/" + filepathNames[i]
			log.Println(tmpFile) //打印path
			_ = os.Remove(tmpFile)
		}
	}

	ctx := c.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("手动停止，等待脚本跑完, 成功退出")
			os.Exit(0)
		}
	}

}

func reload() {
	log.Println("reload")

	if *outputVerbose {
		log.Println("重新加载前的Entrys")
		for _, v := range c.Entries() {
			log.Printf("%#v\n", v)
		}
		log.Println("=============================")
	}

	log.Println("即将打印目前scheduleEntrys")
	for myCmdMd5, cmdAttr := range scheduleEntrys {
		log.Printf("%s, %#v\n", myCmdMd5, cmdAttr)
	}
	log.Println("=============================")

	myKeys := getKeys(scheduleEntrys)

	tmpScheduleMap := parseCrontabFile(*cronConfigFile)
	addCmds := make(map[string]cmdRecord, 0)
	delCmds := make(map[string]cmdRecord, 0)
	newCmdsRecord := make(map[string]cmdRecord, 0)
	for myCmdMd5, cmdAttr := range tmpScheduleMap {
		_, fb := FindSliceVal(myKeys, myCmdMd5)
		if !fb {
			// 需新增的脚本
			addCmds[myCmdMd5] = cmdRecord{EntryID: int(0), Cmd: cmdAttr[1], CmdSpec: cmdAttr[0]}
		} else {
			// 存在脚本，但是spec调度调整的需要删除
			if cmdAttr[0] != scheduleEntrys[myCmdMd5].CmdSpec {
				delCmds[myCmdMd5] = cmdRecord{EntryID: scheduleEntrys[myCmdMd5].EntryID, Cmd: cmdAttr[1], CmdSpec: cmdAttr[0]}
				addCmds[myCmdMd5] = cmdRecord{EntryID: int(0), Cmd: cmdAttr[1], CmdSpec: cmdAttr[0]}
			}
		}
		newCmdsRecord[myCmdMd5] = cmdRecord{EntryID: int(0), Cmd: cmdAttr[1], CmdSpec: cmdAttr[0]}
	}

	newMyKeys := getKeys(newCmdsRecord)
	for myCmdMd5, record := range scheduleEntrys {
		_, fb := FindSliceVal(newMyKeys, myCmdMd5)
		if !fb {
			// 需删除的脚本
			delCmds[myCmdMd5] = record
			delete(scheduleEntrys, myCmdMd5)
		}
	}

	log.Println("addCmds", addCmds)
	log.Println("delCmds", delCmds)

	if len(delCmds) > 0 {
		for _, record := range delCmds {
			log.Println("删除脚本:", record)
			c.Remove(cron.EntryID(record.EntryID))
		}
	}

	if len(addCmds) > 0 {
		for myCmdMd5, record := range addCmds {
			func(myCmdMd5 string, record cmdRecord) {
				log.Println("读取配置item", myCmdMd5, record)
				entryId, err := c.AddFunc(record.CmdSpec, func() {
					runShell(record.Cmd, myCmdMd5)
				})
				if *outputVerbose {
					log.Println("加入调度情况: ", entryId, err)
				}
				scheduleEntrys[myCmdMd5] = record
			}(myCmdMd5, record)

		}
	}

	log.Println("重载后的scheduleEntrys")
	for myCmdMd5, cmdAttr := range scheduleEntrys {
		log.Printf("%s, %#v\n", myCmdMd5, cmdAttr)
	}
	log.Println("=============================")

	if *outputVerbose {
		log.Println("重新加载后的Entrys")
		for _, v := range c.Entries() {
			log.Printf("%#v\n", v)
		}
		log.Println("=============================")
	}
}

func FindSliceVal(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func getKeys(m map[string]cmdRecord) []string {
	// 数组默认长度为map长度,后面append时,不需要重新申请内存和拷贝,效率较高
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

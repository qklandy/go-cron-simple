## introduce
> go单文件尝试版本的类crontab的支持秒级的调度器
> 
> 代码没有做深度的结构化思考，只未实现功能尝试，大家可以参考思路


## install
```
$ git clone https://github.com/qklandy/go-cron-simple

# ./build.sh 目录 main文件 输出的bin文件名 GOOS GOARCH
# 编译linux下的64位的bin文件
$ ./build.sh . main goCrondSimple linux amd64
```

## Usage

### crontab
> @：crontab时间格式和执行命令的分隔符

#### demo
```
* * * * * * @ php artisan c:cmd:hotfix
* * * * * * @ php artisan list
预留一行
```

#### explain
```
*    *    *    *    *    *
-    -    -    -    -    -
|    |    |    |    |    |
|    |    |    |    |    +----- 星期中星期几 (0 - 7) (星期天 为0)
|    |    |    |    +---------- 月份 (1 - 12) 
|    |    |    +--------------- 一个月中的第几天 (1 - 31)
|    |    +-------------------- 小时 (0 - 23)
|    +------------------------- 分钟 (0 - 59)
+------------------------------ 秒   (0 - 59)
```

### fire
```
# simple execute
$ ./goCrondSimple -s=./config/crontab -dd=./ -vvv=true

# background execute
$ nohup ./goCrondSimple -s=./config/crontab -dd=./ -vvv=true >> /dev/null 2>&1 &

# systemd
hold on...
```

## Extended
```
GOOS：目标可执行程序运行操作系统
darwin: macos
freebsd: bsd
linux: linux
windows: window系统

GOARCH：目标可执行程序操作系统构架
386：32位
amd64：64位
arm：arm
```
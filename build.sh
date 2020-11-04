#./build.sh . main lizardCron linux amd64
tmpDir=`date +%Y%m%d%H%M`
mkdir -p ./tpl-${tmpDir}
cp -r ./tpl/ ./tpl-${tmpDir}/
cd ${1}
echo `pwd`
echo "GOOS=${4-linux} GOARCH=${5-amd64} go build -o ${3} ${2}.go"
GOOS=${4-linux} GOARCH=${5-amd64} go build -o ${3} ${2}.go
mv ./${3} ./tpl-${tmpDir}/
tar zcvf ${tmpDir}.gz ./tpl-${tmpDir}

read -r -p "是否删除临时目录? [Y/n], 默认为Y自动删除 " input
case $input in
    [yY][eE][sS]|[yY][""])
    rm -rf ./tpl-${tmpDir}
		;;

    [nN][oO]|[nN])
		echo "已为你保留临时目录"
		ls -lh ./tpl-${tmpDir}/${3}
		;;

    *)
		rm -rf ./tpl-${tmpDir}
		exit 0
		;;
esac
#if [ $? -eq 0 ]; then
#  echo "success"
#else
#  echo "failure"
#fi
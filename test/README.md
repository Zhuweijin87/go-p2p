### p2p 的简单例子 

1. 下面配置自行根据实际情况修改:   
```go
var Mainnet = []node{
	{"192.168.1.105", 8001, 8001, "111111"},
}
``` 

2. 启动默认节点:   
```go 
go run main.go -d
``` 

3. 启动其他节点:   
```go
go run main.go -host xxx.xxx.xxx.xxx -port xxxx -id xxxxxx
```

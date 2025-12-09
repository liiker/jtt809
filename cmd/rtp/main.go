package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/zboyco/jtt809/pkg/jtt1078"
)

var addr = flag.String("addr", ":8080", "监听地址")

func main() {
	flag.Parse()

	// 创建视频转码服务器实例
	s := jtt1078.NewVideoServer(*addr)

	// 设置信号处理，优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务器（阻塞）
	go func() {
		if err := s.Start(); err != nil {
			log.Fatal(err)
		}
	}()

	// 等待退出信号
	<-sigChan
	fmt.Println("\n🛑 收到退出信号，正在关闭服务器...")
}

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"revisiongate/internal/application"
	"revisiongate/internal/selfcheck"
	"revisiongate/internal/store"
	"revisiongate/internal/webui"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Printf("RevisionGate 启动失败：%v", err)
		os.Exit(1)
	}
}
func run() error {
	defaultAddr := "127.0.0.1:19081"
	if fromEnv, err := addressFromPort(os.Getenv("PORT")); err != nil {
		return err
	} else if fromEnv != "" {
		defaultAddr = fromEnv
	}
	addr := flag.String("addr", defaultAddr, "回环监听地址")
	dataDir := flag.String("data", "./revisiongate-data", "事件日志与投影目录")
	check := flag.Bool("selfcheck", false, "通过真实 HTTP 执行有界自检")
	flag.Parse()
	if err := validateAddress(*addr); err != nil {
		return err
	}
	if *check {
		if err := selfcheck.Run(*addr); err != nil {
			return err
		}
		fmt.Println("RevisionGate selfcheck 通过：完整流程、幂等、并发冲突、冻结通知和恢复校验均正常")
		return nil
	}
	repo, err := store.Open(*dataDir)
	if err != nil {
		return fmt.Errorf("打开持久化存储: %w", err)
	}
	defer repo.Close()
	handler := webui.New(application.NewService(repo)).Handler()
	server := &http.Server{Addr: *addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		<-signals
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	log.Printf("RevisionGate 已启动：http://%s", *addr)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

package selfcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"revisiongate/internal/application"
	"revisiongate/internal/domain"
	"revisiongate/internal/store"
	"revisiongate/internal/webui"
	"time"
)

func Run(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "revisiongate-selfcheck-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	repo, err := store.Open(dir)
	if err != nil {
		return fmt.Errorf("打开自检存储: %w", err)
	}
	service := application.NewService(repo)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		_ = repo.Close()
		return fmt.Errorf("监听 %s: %w", addr, err)
	}
	server := &http.Server{Handler: webui.New(service).Handler(), ReadHeaderTimeout: 3 * time.Second, IdleTimeout: 5 * time.Second}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	client := &client{base: "http://" + listener.Addr().String(), http: &http.Client{Timeout: 3 * time.Second}}
	if err = client.waitHealthy(ctx); err == nil {
		var item *domain.RevisionCase
		item, err = runWorkflow(ctx, client)
		if err == nil {
			err = verifyRecovery(repo, dir, item)
		}
	}
	shutdownCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	shutdownErr := server.Shutdown(shutdownCtx)
	serveErr := <-serveDone
	if serveErr == http.ErrServerClosed {
		serveErr = nil
	}
	if closeErr := repo.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err == nil && shutdownErr != nil {
		err = shutdownErr
	}
	if err == nil && serveErr != nil {
		err = serveErr
	}
	return err
}
func verifyRecovery(opened *store.FileStore, dir string, item *domain.RevisionCase) error {
	if err := opened.Close(); err != nil {
		return err
	}
	recovered, err := store.Open(dir)
	if err != nil {
		return fmt.Errorf("重启恢复失败: %w", err)
	}
	defer recovered.Close()
	got, err := recovered.Get(context.Background(), item.ID)
	if err != nil {
		return err
	}
	if got.Version != item.Version || got.Notice == nil || !domain.VerifyNotice(got) {
		return fmt.Errorf("恢复后的冻结通知不一致")
	}
	audits, err := recovered.Audit(context.Background(), item.ID)
	if err != nil {
		return err
	}
	if len(audits) < 10 {
		return fmt.Errorf("审计轨迹不完整: %d", len(audits))
	}
	return nil
}

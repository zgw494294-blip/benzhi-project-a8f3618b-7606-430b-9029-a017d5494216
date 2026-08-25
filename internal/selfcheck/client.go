package selfcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"revisiongate/internal/domain"
	"time"
)

type client struct {
	base string
	http *http.Client
}
type responseEnvelope struct {
	Data     json.RawMessage `json:"data"`
	Replayed bool            `json:"replayed"`
	Error    *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *client) request(ctx context.Context, method, path string, body any, out any) (bool, int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return false, 0, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return false, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return false, res.StatusCode, err
	}
	var env responseEnvelope
	if err = json.Unmarshal(raw, &env); err != nil {
		return false, res.StatusCode, fmt.Errorf("响应 JSON 无效: %w", err)
	}
	if res.StatusCode >= 400 {
		message := "HTTP 操作失败"
		if env.Error != nil {
			message = env.Error.Code + ": " + env.Error.Message
		}
		return env.Replayed, res.StatusCode, fmt.Errorf("%s", message)
	}
	if out != nil && len(env.Data) > 0 {
		if err = json.Unmarshal(env.Data, out); err != nil {
			return env.Replayed, res.StatusCode, err
		}
	}
	return env.Replayed, res.StatusCode, nil
}
func (c *client) waitHealthy(ctx context.Context) error {
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/healthz", nil)
		if err == nil {
			if res, e := c.http.Do(req); e == nil {
				_ = res.Body.Close()
				if res.StatusCode == 200 {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待健康检查: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
func postCase(c *client, ctx context.Context, path string, body any) (*domain.RevisionCase, bool, error) {
	var item domain.RevisionCase
	replayed, _, err := c.request(ctx, http.MethodPost, path, body, &item)
	return &item, replayed, err
}

package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// versionKey 把 ExpectedVersion 归一为字符串，纳入请求指纹，
// 这样带不同期望版本的请求不会互相重放。
func versionKey(v int64) string {
	return fmt.Sprintf("%d", v)
}

// fingerprintCtxKey 携带应用层为写请求计算的请求指纹，
// 供存储层在重放时核对操作与请求内容是否一致。
type fingerprintCtxKey struct{}

// withFingerprint 将请求指纹注入上下文，供存储层读取。
func withFingerprint(ctx context.Context, fingerprint string) context.Context {
	return context.WithValue(ctx, fingerprintCtxKey{}, fingerprint)
}

// FingerprintFromContext 取出上下文中携带的请求指纹；不存在时返回空串。
func FingerprintFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(fingerprintCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// requestFingerprint 为一次写请求计算稳定摘要。
// 它同时编码操作类型与规范化后的请求内容，使同一幂等键
// 重放完全相同的写请求时返回原结果，而用于不同操作或不同请求内容时
// 能够被识别并拒绝。
func requestFingerprint(action, expectedVersion, pathTarget string, content any) string {
	var parts []string
	parts = append(parts, action)
	if expectedVersion != "" {
		parts = append(parts, "v="+expectedVersion)
	}
	if strings.TrimSpace(pathTarget) != "" {
		parts = append(parts, "t="+strings.TrimSpace(pathTarget))
	}
	parts = append(parts, canonicalContent(content))
	raw := strings.Join(parts, "\x1f")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// canonicalContent 把任意结构体编码为键有序的 JSON，忽略幂等键与操作人等客户端标签。
// 传入 nil 或空内容时返回空串，使摘要仍能区分操作与版本。
func canonicalContent(content any) string {
	if content == nil {
		return ""
	}
	raw, err := json.Marshal(content)
	if err != nil {
		return ""
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		// 非对象（数组/字符串等）时直接使用规范化后的原始 JSON。
		return string(raw)
	}
	stripped := stripClientMeta(value)
	return string(canonicalEncode(stripped))
}

// stripClientMeta 移除不影响业务结果但属于客户端标签的字段，
// 例如幂等键与操作人；其余字段保持不变。
func stripClientMeta(value map[string]any) map[string]any {
	out := make(map[string]any, len(value))
	for k, v := range value {
		lk := strings.ToLower(k)
		if lk == "idempotencykey" || lk == "actor" {
			continue
		}
		out[k] = v
	}
	return out
}

func canonicalEncode(value any) []byte {
	buf := &byteBuffer{}
	encodeCanonical(buf, value)
	return buf.Bytes()
}

type byteBuffer struct{ data []byte }

func (b *byteBuffer) Write(p []byte) (int, error) { b.data = append(b.data, p...); return len(p), nil }
func (b *byteBuffer) WriteString(s string)        { b.data = append(b.data, s...) }
func (b *byteBuffer) writeByte(c byte)            { b.data = append(b.data, c) }
func (b *byteBuffer) Bytes() []byte               { return b.data }

func encodeCanonical(buf *byteBuffer, value any) {
	switch v := value.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if v {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case json.Number:
		buf.WriteString(v.String())
	case float64:
		buf.WriteString(jsonNumber(v))
	case int:
		buf.WriteString(jsonNumber(float64(v)))
	case int64:
		buf.WriteString(jsonNumber(float64(v)))
	case string:
		raw, _ := json.Marshal(v)
		buf.Write(raw)
	case []any:
		buf.writeByte('[')
		for i, item := range v {
			if i > 0 {
				buf.writeByte(',')
			}
			encodeCanonical(buf, item)
		}
		buf.writeByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.writeByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.writeByte(',')
			}
			kb, _ := json.Marshal(k)
			buf.Write(kb)
			buf.writeByte(':')
			encodeCanonical(buf, v[k])
		}
		buf.writeByte('}')
	default:
		raw, _ := json.Marshal(v)
		buf.Write(raw)
	}
}

func jsonNumber(n float64) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}

# RevisionGate

RevisionGate 是面向维修文件编制员、技术校核员和车间放行负责人的航空器维修手册临时修订工作台。它管理修订建档、有序条款变更、工程依据追溯、自动规则校核、技术问题整改复核、负责人批准，以及带校验码的不可变车间生效通知。

服务提供原生 HTML、CSS 和 JavaScript 浏览器页面，页面只调用同源 JSON HTTP 接口。数据以本地长度前缀事件日志和原子投影快照保存；事件具有递增序号、前序摘要和校验和，启动时会验证并恢复状态。所有写请求使用 `idempotencyKey`，任务创建后的状态变更还必须携带 `expectedVersion`。

## 构建

```text
go build ./cmd/server
```

## 运行

```text
go run ./cmd/server -addr=127.0.0.1:19081 -data=./revisiongate-data
```

执行会真实启动 HTTP 监听并自动退出的完整自检：

```text
go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
```

## 测试

```text
go test ./...
```

服务默认仅监听 `127.0.0.1:19081`，可通过 `-addr=127.0.0.1:<port>` 覆盖，或令 `PORT` 为端口号后绑定 `127.0.0.1:<PORT>`。程序拒绝非回环地址和低于 `1024` 的端口。浏览器入口为 `GET /`，健康检查为 `GET /healthz`。

主要 API 位于 `/api/cases`。任务详情响应包含当前轮有序变更、按规则分组的失败定位、内容摘要绑定的送审就绪清单、当前整改轮复核队列、允许操作和通知校验状态。草拟或整改中的变更块可通过 `PUT /api/cases/{id}/changes/{blockId}`、`DELETE /api/cases/{id}/changes/{blockId}` 和 `POST /api/cases/{id}/changes/reorder` 修订；成功操作会使原校核结果失效。复核队列与原子批量结论分别使用 `GET /api/cases/{id}/findings/queue` 和 `POST /api/cases/{id}/findings/batch-review`。

只读的生效通知检索入口为 `GET /api/notices`，支持 `serialNumber`、`manualNumber`、`aircraftModel`、`configuration`、`verificationCode`、`page` 和 `pageSize`。`GET /api/notices/{noticeId}/verify?verificationCode=...` 会重新计算冻结摘要，并返回当前有效、已过期、完整性异常或校验码未匹配结论。检索和核验不会改变任务版本或审计链。生效任务无法再次修改，审计事件可通过 `GET /api/cases/{id}/audit` 查看。

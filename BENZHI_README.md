# BENZHI_README

基于 Go 实现的RevisionGate Web 项目，一款后端服务，RevisionGate 已完整实现航空器维修手册临时修订工作台，覆盖修订建档、有序条款编制、依据与适用性校核、送审锁定、问题整改复核、批准冻结、生效通知签发、审计追踪、并发控制、幂等写入和真实 HTTP 自检。

## 项目说明
- 项目：benzhi-project-a8f3618b-7606-430b-9029-a017d5494216
- 项目用途：RevisionGate 已完整实现航空器维修手册临时修订工作台，覆盖修订建档、有序条款编制、依据与适用性校核、送审锁定、问题整改复核、批准冻结、生效通知签发、审计追踪、并发控制、幂等写入和真实 HTTP 自检。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-a8f3618b-7606-430b-9029-a017d5494216-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-a8f3618b-7606-430b-9029-a017d5494216-arm64 linux/arm64
docker run -it benzhi-project-a8f3618b-7606-430b-9029-a017d5494216-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selfcheck -addr=127.0.0.1:19081`

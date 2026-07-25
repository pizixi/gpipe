# gpipe

`gpipe` 是 Rust `npipe` 的 Go 重构版本，尽量保持原有业务与协议兼容，并提供可独立构建、部署的服务端、客户端和 Web 管理端。

## 功能

- 客户端登录、自动重连、心跳保活和隧道动态同步
- 用户、玩家和隧道的 Web 管理
- 本地/远端 `TCP`、`UDP`、`SOCKS5`、`HTTP` 代理
- `tcp://`、`ws://`、`quic://`、`kcp://` 传输
- 可选 TLS、Shadowsocks 出站和 `illegal_traffic_forward` 非协议流量转发
- 纯 Go SQLite 存储，无 CGO 依赖
- Web 前端内置于服务端二进制，也可通过 `web_base_dir` 覆盖
- Windows / Linux 系统服务安装与卸载
- 玩家客户端生成、版本上报和跨平台远程升级
- 升级包断点续传、SHA-256/HMAC 校验、健康检查和失败自动回滚

## 快速开始

从源码构建需要 Go、Node.js/npm 和 PowerShell；运行已经打包好的发布目录不需要 Go 或 Node.js。

### 1. 构建发布包

```powershell
.\scripts\build-release.ps1 -OutputDir .\release -Version 1.1.0 -Clean
```

`1.1.0` 是示例版本。正式发布时必须显式填写并递增语义版本（SemVer）。脚本会将同一个版本写入客户端二进制、`release/gpipe.json` 和 `release/client-templates/manifest.json`。

### 2. 检查配置并启动服务端

至少应修改 `release/gpipe.json` 中的 Web 管理密码，并按部署环境检查监听地址、TLS 和客户端版本配置。

```powershell
Set-Location .\release
.\gpipe-server.exe -config-file .\gpipe.json
```

默认 Web 管理地址为 `http://127.0.0.1:8120`。Linux amd64 可运行发布目录中的 `gpipe-server-linux-amd64`。

### 3. 创建玩家并运行客户端

在 Web 管理端创建玩家后，可直接生成对应平台的客户端。源码开发时也可以手工启动通用客户端：

```powershell
.\bin\gpipe-client.exe run --server tcp://127.0.0.1:8118 --key demo
```

## 构建与发布

### 一键发布脚本

```powershell
.\scripts\build-release.ps1 -OutputDir .\release -Version 1.1.0 -Clean
```

脚本依次完成：

1. 安装缺失的前端依赖并构建 `frontend/dist/`。
2. 将前端静态资源（包括 `favicon.ico`）嵌入服务端。
3. 构建当前宿主平台服务端和 Linux amd64 服务端。
4. 构建 Windows/Linux 多架构客户端模板及 `manifest.json`。
5. 复制并规范化 `gpipe.json`，同步 `client_latest_version`。
6. 创建数据库、缓存和日志目录；仓库存在 `certs/` 时一并复制。

常用参数：

| 参数 | 说明 |
| --- | --- |
| `-OutputDir` | 发布目录，默认 `./release` |
| `-ConfigPath` | 源配置文件，默认 `./gpipe.json` |
| `-Version` | 客户端语义版本，正式发布时应显式指定 |
| `-ServerGOOS` / `-ServerGOARCH` | 指定主服务端产物的平台和架构 |
| `-SkipFrontend` | 跳过前端构建，沿用现有 `frontend/dist` |
| `-SkipTemplates` | 跳过客户端模板构建 |
| `-SkipCerts` | 不复制证书目录 |
| `-Clean` | 构建前清理指定发布目录 |

修改过页面、图标、CSS 或 JavaScript 时不要使用 `-SkipFrontend`，否则这些改动不会进入新服务端。

典型发布目录：

```text
release/
  gpipe-server.exe
  gpipe-server-linux-amd64
  gpipe.json
  gpipe.db
  logs/
  client-cache/
  client-templates/
    manifest.json
    gpipe-client-template-windows-amd64.exe
    gpipe-client-template-windows-arm64.exe
    gpipe-client-template-linux-amd64
    gpipe-client-template-linux-arm64
    gpipe-client-template-linux-armv7
  certs/                         # 仅启用 TLS 时需要
```

### 手工构建

前端有改动时先构建前端，再构建服务端：

```powershell
Set-Location .\frontend
npm ci
npm run build
Set-Location ..

go build -ldflags "-s -w" -buildvcs=false -o .\bin\gpipe-server.exe .\cmd\server
go build -ldflags "-s -w" -buildvcs=false -o .\bin\gpipe-client.exe .\cmd\client
```

单独构建纯发布版客户端模板：

```powershell
.\scripts\build-client-templates.ps1 -OutputDir .\client-templates -Version 1.1.0
```

Linux 客户端交叉构建示例：

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -ldflags "-s -w" -buildvcs=false -o .\bin\gpipe-client-linux-amd64 .\cmd\client
```

## 服务端配置

启动命令：

```powershell
.\gpipe-server.exe -config-file .\gpipe.json
```

默认配置文件是 `gpipe.json`。未传 `-config-file` 且当前目录只有旧版 `config.json` 时，服务端会兼容读取旧文件名。

完整示例：

```json
{
  "database_url": "sqlite://gpipe.db?mode=rwc",
  "listen_addr": "tcp://0.0.0.0:8118,kcp://0.0.0.0:8118,ws://0.0.0.0:8119,quic://0.0.0.0:8119",
  "illegal_traffic_forward": "",
  "enable_tls": false,
  "tls_cert": "./certs/cert.pem",
  "tls_key": "./certs/server.key.pem",
  "web_base_dir": "",
  "web_addr": "0.0.0.0:8120",
  "web_username": "admin",
  "web_password": "admin@1234",
  "client_template_dir": "./client-templates",
  "client_artifact_cache_dir": "./client-cache",
  "client_latest_version": "1.1.0",
  "quiet": false,
  "log_dir": "logs"
}
```

| 配置项 | 说明 |
| --- | --- |
| `database_url` | SQLite 地址，例如 `sqlite://gpipe.db?mode=rwc` |
| `listen_addr` | 客户端服务监听地址，多个地址用英文逗号分隔 |
| `illegal_traffic_forward` | 非 npipe 协议流量转发目标，例如 `127.0.0.1:80` |
| `enable_tls` | 是否加密客户端与服务端之间的传输链路 |
| `tls_cert` / `tls_key` | 服务端 TLS 证书和私钥 |
| `web_base_dir` | 可选的磁盘 Web 资源目录；为空或不存在时使用内置页面 |
| `web_addr` | Web 管理端监听地址 |
| `web_username` / `web_password` | Web 管理凭据；任一留空都会关闭 Web 管理端 |
| `client_template_dir` | 预构建客户端模板目录 |
| `client_artifact_cache_dir` | 已注入玩家配置的客户端下载缓存目录 |
| `client_latest_version` | 最新客户端语义版本，应与模板 manifest 一致 |
| `quiet` | 是否静默运行 |
| `log_dir` | 日志目录 |

`web_base_dir` 中既可以放完整 `index.html`，也可以放 `templates/` 目录使用模板渲染，适合前端联调。

## 客户端生成与远程升级

### 玩家客户端下载

Web 管理端按以下顺序生成玩家客户端：

1. 优先读取 `client_template_dir` 中的预构建模板，将玩家密钥、服务端地址、TLS 和 Shadowsocks 参数注入二进制。
2. 找不到模板时，回退到源码目录并调用 `go build` 动态编译。

因此，正式部署应携带 `client-templates/`。这样服务端机器无需 Go 工具链，也能生成 Windows/Linux 客户端；`client_artifact_cache_dir` 可减少相同客户端的重复生成开销。

### 版本规则

- `build-release.ps1 -Version` 会同步发布配置、客户端二进制和模板 manifest 的版本。
- `client_latest_version` 必须与 `client-templates/manifest.json` 的 `version` 完全一致。
- 旧配置缺少 `client_latest_version` 时，新服务端会从 manifest 自动读取；正式部署仍建议显式配置。
- 显式配置与 manifest 不一致时，普通客户端下载仍可使用，但远程升级会被禁用，避免分发错误版本。
- 不支持远程降级：客户端版本高于服务端目标版本时不会升级。

玩家列表使用单行版本徽标：最新版本显示绿色，可升级版本以琥珀色显示“当前版本 → 目标版本”。平台、目标版本、升级进度和失败原因通过鼠标悬浮提示展示。

升级按钮只有在以下条件全部满足时才启用：

- 客户端在线并支持安全升级协议
- 客户端平台受支持，版本号有效且低于目标版本
- 没有其他升级任务正在执行
- 存在版本、平台和校验信息均匹配的客户端模板

按钮被禁用时，悬浮提示会区分离线、不支持升级、平台未知、版本无效、已是最新、客户端版本更高、任务执行中和升级产物不可用等原因。

### 安全与平滑升级

- 升级包通过现有客户端控制连接分块传输，不要求客户端访问 Web 管理端口。
- 分块支持断线续传；完整包通过 SHA-256 和玩家密钥 HMAC 双重验证后才允许切换。
- Windows 使用独立升级助手等待旧进程退出后替换文件，避免覆盖运行中的可执行文件。
- Linux/其他类 Unix 系统使用备份后切换的方式替换文件。
- 旧二进制保留到新版本重新登录成功；120 秒健康检查失败会恢复备份并重启旧版本。
- 客户端进程必须对自身目录具有写权限，系统服务安装通常已满足这一条件。

加入自更新协议之前构建的旧客户端不会声明升级能力，需要先手工替换一次；之后可通过 Web 管理端跨版本升级。

## 客户端使用

### 命令行

普通连接：

```powershell
.\bin\gpipe-client.exe run --server tcp://127.0.0.1:8118 --key demo
```

启用 TLS：

```powershell
.\bin\gpipe-client.exe run --server quic://127.0.0.1:8119 --key demo --enable-tls
```

通过 Shadowsocks 连接：

```powershell
.\bin\gpipe-client.exe run --server ws://127.0.0.1:8119 --key demo --ss-server 127.0.0.1:8388 --ss-method chacha20-ietf-poly1305 --ss-password your-password
```

常用参数：

| 参数 | 说明 |
| --- | --- |
| `--server` | 服务端地址；多个地址用英文逗号分隔 |
| `--key` | 玩家密钥 |
| `--enable-tls` | 启用客户端与服务端之间的 TLS |
| `--tls-server-name` | TLS SNI |
| `--ss-server` / `--ss-method` / `--ss-password` | Shadowsocks 出站配置，三项需同时提供 |
| `--quiet` | 静默模式 |
| `--log-dir` | 日志目录 |
| `--backtrace` | 输出更完整的运行时回溯 |

兼容说明：

- `--ca-cert` 和 `--insecure` 仍可解析，用于兼容旧命令行或旧服务参数。
- 当前客户端在 TLS 模式下默认跳过证书链和主机名校验，通常无需传 `--ca-cert`。
- Shadowsocks 自定义拨号目前仅支持 `tcp://` 和 `ws://`；`quic://`、`kcp://` 使用原生 UDP 拨号。

### 安装系统服务

Windows 和 Linux 均支持：

```powershell
.\bin\gpipe-client.exe install --server tcp://127.0.0.1:8118 --key demo
.\bin\gpipe-client.exe uninstall
```

- Windows 通常需要管理员权限。
- Linux 通常需要 `systemd` 和足够权限。
- `run-service` 供服务管理器调用，一般不应手工执行。

## TLS 范围与本地证书

`enable_tls` 只保护 `gpipe client <-> gpipe server` 这一段链路。隧道内承载的 TCP、UDP、SOCKS5 和 HTTP 数据在这一段会被加密，但本地应用到客户端、服务端到最终目标的链路是否加密，取决于业务程序本身。

还需注意：

- Web 管理端 `web_addr` 是独立的 HTTP 接口，不受 `enable_tls` 控制。
- `quic://` 本身要求 TLS；`tcp://`、`ws://`、`kcp://` 可通过 `enable_tls` 加密。
- 当前 TLS 默认只提供链路加密，不校验证书链和主机名；`--tls-server-name` 仍可指定 SNI。

生成本地调试证书：

```powershell
.\generate-certificate.ps1 -Force
```

证书输出到 `certs/`：

- `cert.pem`、`server.key.pem`：服务端证书和私钥
- `root-ca.pem`、`root-ca.key.pem`：本地根证书和私钥
- 默认 SAN：`localhost`、`127.0.0.1`、`::1`

## 作为 Go 包调用

第三方 Go 程序应导入公开包 `github.com/pizixi/gpipe/client`，不要导入 `internal/client`。

```go
package main

import (
  "context"
  "log"
  "os"
  "os/signal"

  gclient "github.com/pizixi/gpipe/client"
)

func main() {
  ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
  defer stop()

  if err := gclient.RunContext(ctx, gclient.Options{
    Server: "tcp://127.0.0.1:8118",
    Key:    "demo",
    Logger: log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile),
  }); err != nil {
    log.Fatal(err)
  }
}
```

Shadowsocks 拨号辅助：

```go
dial, err := gclient.NewShadowsocksDialFunc(gclient.SSDialConfig{
  ServerAddr: "127.0.0.1:8388",
  Method:     "chacha20-ietf-poly1305",
  Password:   "your-password",
})
if err != nil {
  log.Fatal(err)
}
```

更多示例：

- `examples/third_party_go_client`：独立 Go 模块及可选 Shadowsocks 出站示例
- `examples/client_direct_demo.go`：仓库内直接运行的完整示例

## 验证

最小链路验证：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke.ps1
```

该脚本会生成临时配置，启动服务端，验证内置 Web 资源，通过 API 创建测试用户，并启动客户端验证登录。

Windows 真实跨版本升级演练：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\upgrade-smoke.ps1
```

该脚本构建 `1.0.0` 客户端并通过管理 API 升级到 `1.1.0`，验证文件替换、重新登录、任务状态和残留文件清理。

常规代码检查：

```powershell
go test ./...
go vet ./...

Set-Location .\frontend
npm run build
npm audit
```

## 项目结构

| 路径 | 用途 |
| --- | --- |
| `cmd/server` / `cmd/client` | 服务端和客户端入口 |
| `client` | 对第三方 Go 程序公开的客户端包 |
| `internal/server` / `internal/client` | 服务端与客户端核心逻辑 |
| `internal/proxy` | 代理入口、出口、加密、压缩和数据转发 |
| `internal/codec` / `internal/proto` | 帧与协议消息编解码 |
| `internal/db` / `internal/web` | SQLite 存储与 Web 管理 API |
| `frontend` | React 管理端；产物写入 `frontend/dist` |
| `scripts/build-release.ps1` | 一键发布构建 |
| `scripts/build-client-templates.ps1` | 多平台客户端模板构建 |
| `scripts/smoke.ps1` / `scripts/upgrade-smoke.ps1` | 基础链路与升级验证 |
| `examples` | Go 包调用示例 |

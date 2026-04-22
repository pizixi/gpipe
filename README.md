# gpipe

`gpipe` 是对 Rust `npipe` 项目的 Go 重构版本，目标是在业务逻辑上尽量与原项目保持一致，并提供可独立构建、运行和部署的 Go 实现。

当前版本已经包含控制面、Web 管理端、隧道同步、代理转发、`TCP / WS / QUIC / KCP` 传输、`TCP / UDP / SOCKS5 / HTTP` 代理类型，以及基于纯 Go SQLite 驱动的服务端存储实现。

## 项目结构

- `cmd/server`：服务端入口
- `cmd/client`：客户端入口
- `client`：对第三方 Go 程序公开的客户端调用包
- `examples/third_party_go_client`：独立第三方 Go 模块调用客户端的示例，支持源码变量形式的可选 Shadowsocks 出站
- `internal/client`：客户端主循环、登录、心跳、隧道同步
- `internal/server`：服务端主逻辑、连接管理、协议处理
- `internal/proxy`：代理入口、出口、数据转发、加密与压缩
- `internal/codec`：外层帧编解码
- `internal/proto`：与 Rust 版本兼容的消息编解码
- `internal/db`：SQLite 初始化与迁移
- `internal/web`：管理后台 HTTP API
- `scripts/build-release.ps1`：一键生成发布目录
- `frontend/`：React 管理端源码，构建产物输出到 `frontend/dist` 并打进服务端二进制
- `scripts/build-client-templates.ps1`：预构建客户端下载模板
- `scripts/smoke.ps1`：本地最小链路验证脚本

## 已实现特性

- 纯 Go SQLite 驱动，使用 `modernc.org/sqlite`
- 服务端 Web 管理接口
- 用户与隧道的增删改查
- 客户端登录、重连、心跳保活
- 隧道变更通知与动态同步
- 本地和远端的 `TCP / UDP / SOCKS5 / HTTP` 代理
- 传输协议支持：
  - `tcp://`
  - `ws://`
  - `quic://`
  - `kcp://`
- `illegal_traffic_forward` 非协议流量转发
- 默认管理端页面已内置到服务端二进制中
- `web_base_dir` 可选磁盘目录覆盖，便于前端联调
- Windows / Linux 服务安装与卸载

## 构建

建议在项目目录内使用独立缓存目录，避免污染全局 Go 缓存。

```powershell
go build -ldflags "-s -w" -buildvcs=false -o .\bin\gpipe-server.exe .\cmd\server
go build -ldflags "-s -w" -buildvcs=false -o .\bin\gpipe-client.exe .\cmd\client
```

如果要启用传输层 TLS，请在 `gpipe` 目录生成证书：

```powershell
.\generate-certificate.ps1 -Force
```

生成结果默认输出到 `.\certs\`：

- `.\certs\cert.pem`
- `.\certs\server.key.pem`
- `.\certs\root-ca.pem`
- `.\certs\root-ca.key.pem`

发布包最小建议内容：

- `bin/gpipe-server(.exe)`
- `bin/gpipe-client(.exe)`
- `gpipe.json`
- `certs/` 目录（仅在 `enable_tls=true` 时需要）

如果你希望 Web 后台在“纯发布版环境”里直接为玩家生成客户端下载，而不依赖本机 Go 工具链和源码目录，额外建议携带：

- `client-templates/` 目录

说明：

- `frontend/dist` 已内置进服务端二进制，发布时不需要再额外携带这个目录
- 如果设置了 `web_base_dir` 且目录存在，服务端会优先使用磁盘目录中的静态文件；目录里既可以放完整 `index.html`，也可以放 `templates/` 目录让首页走模板渲染
- `client-templates/` 可通过 `.\scripts\build-client-templates.ps1` 预先构建；服务端下载玩家客户端时会优先使用这些模板，并把玩家密钥和连接参数直接补丁进二进制

纯发布版模板构建示例：

```powershell
.\scripts\build-client-templates.ps1 -OutputDir .\client-templates
```

一键生成发布目录：

```powershell
.\scripts\build-release.ps1 -OutputDir .\release -Clean
```

这个脚本会：

- 先构建前端到 `frontend/dist/`
- 把前端静态资源一并嵌入随后生成的服务端二进制；包括 `favicon.ico` 在内的页面资源都会随二进制发布
- 构建服务端二进制到 `release/gpipe-server.exe`（Linux 下为 `release/gpipe-server`）
- 构建客户端模板到 `release/client-templates/`
- 复制并规范化 `release/gpipe.json`
- 创建 `release/client-cache/`、`release/logs/`、`release/gpipe.db`
- 如果仓库里存在 `certs/`，自动复制到发布目录

如果你修改过前端页面、图标、CSS、JS 等静态资源，不要加 `-SkipFrontend`；否则会沿用上一次的 `frontend/dist` 构建结果，新的 `favicon.ico` 不会进入本次服务端二进制。

Linux 交叉构建客户端示例：

```powershell
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -ldflags "-s -w" -buildvcs=false -o .\bin\gpipe-client-linux-amd64 .\cmd\client
```

## 服务端运行

```powershell
.\gpipe-server.exe -config-file .\gpipe.json
```

默认配置文件名已经调整为 `gpipe.json`。为了兼容旧部署，如果你没有显式传 `-config-file`，且当前目录只有旧文件名 `config.json`，服务端仍会自动回退读取它。

服务端配置示例：

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
  "quiet": false,
  "log_dir": "logs"
}
```

配置项说明：

| 配置项                      | 说明                                                               |
| --------------------------- | ------------------------------------------------------------------ |
| `database_url`              | 数据库地址，目前只支持 SQLite，示例：`sqlite://gpipe.db?mode=rwc`  |
| `listen_addr`               | 服务监听地址，多个地址用英文逗号分隔                               |
| `illegal_traffic_forward`   | 非 `npipe` 协议流量转发目标，例如 `127.0.0.1:80`                   |
| `enable_tls`                | 是否为客户端 `<->` 服务端传输链路启用 TLS                          |
| `tls_cert`                  | TLS 证书路径                                                       |
| `tls_key`                   | TLS 私钥路径                                                       |
| `web_base_dir`              | 可选磁盘静态资源目录；为空或目录不存在时回退到二进制内置的页面     |
| `web_addr`                  | Web 管理端监听地址                                                 |
| `web_username`              | Web 管理账号，留空则关闭 Web 管理                                  |
| `web_password`              | Web 管理密码，留空则关闭 Web 管理                                  |
| `client_template_dir`       | 可选的客户端模板目录；存在目标模板时，下载玩家客户端不需要 Go 环境 |
| `client_artifact_cache_dir` | 可选的客户端下载缓存目录；缓存已补丁好的玩家专属二进制             |
| `quiet`                     | 是否静默运行                                                       |
| `log_dir`                   | 日志目录                                                           |

## 纯发布版客户端下载

Web 后台“生成客户端”现在支持两种工作模式：

1. 优先读取 `client_template_dir` 下的预构建模板，直接把玩家密钥、服务端地址、TLS、Shadowsocks 参数补丁进二进制。
2. 如果没有找到模板，再回退到源码目录 + `go build` 动态编译。

推荐发布方式：

- 先在构建机执行 `.\scripts\build-client-templates.ps1`
- 把生成的 `client-templates/` 目录和服务端程序一起发布
- 在 `gpipe.json` 里设置 `client_template_dir`
- 如需减少重复 I/O，再额外配置 `client_artifact_cache_dir`

这样发布机即使没有安装 Go，也可以正常在玩家列表里生成 Windows / Linux 客户端下载。

如果你希望直接一次命令产出完整发布目录，推荐直接执行：

```powershell
.\scripts\build-release.ps1 -OutputDir .\release -Clean
```

最小发布目录结构示例：

```text
release/
  gpipe-server.exe
  gpipe.json
  gpipe.db
  logs/
  client-cache/
  client-templates/
    gpipe-client-template-windows-amd64.exe
    gpipe-client-template-windows-arm64.exe
    gpipe-client-template-linux-amd64
    gpipe-client-template-linux-arm64
    gpipe-client-template-linux-armv7
  certs/
    cert.pem
    server.key.pem
```

说明：

- `client-templates/` 是纯发布版环境里网页“生成客户端”功能的关键目录
- `client-cache/` 可以预先创建，也可以让服务端首次运行时自动创建
- `certs/` 只有在 `enable_tls=true` 时才需要
- 如果是 Linux 发布，把根目录下的 `gpipe-server.exe` 换成对应的 `gpipe-server` 文件名即可

## 客户端运行

前台运行：

```powershell
.\bin\gpipe-client.exe run --server tcp://127.0.0.1:8118 --key demo
```

启用 TLS 的示例：

```powershell
.\bin\gpipe-client.exe run --server quic://127.0.0.1:8119 --key demo --enable-tls
```

常用参数：

- `--server`：服务端地址，支持多个地址，使用英文逗号分隔
- `--key`：玩家密钥
- `--enable-tls`：为客户端 `<->` 服务端传输链路启用 TLS
- `--tls-server-name`：TLS SNI
- `--ss-server`：可选，Shadowsocks 服务端地址
- `--ss-method`：可选，Shadowsocks 加密方式
- `--ss-password`：可选，Shadowsocks 密码
- `--quiet`：静默模式
- `--log-dir`：日志目录
- `--backtrace`：启用更完整的运行时回溯

兼容性说明：

- `--ca-cert` 和 `--insecure` 仍保留解析，主要用于兼容旧命令行/旧服务参数
- 当前客户端在 TLS 模式下默认跳过证书校验，TLS 只用于链路加密，因此通常不需要再传 `--ca-cert`
- 当同时传入 `--ss-server`、`--ss-method`、`--ss-password` 时，客户端会通过 Shadowsocks 出站连接服务端
- 目前自定义拨号只支持 `tcp://` 和 `ws://` 服务端地址；`quic://`、`kcp://` 仍然使用原生 UDP 拨号

通过 Shadowsocks 连接服务端示例：

```powershell
.\bin\gpipe-client.exe run --server ws://127.0.0.1:8119 --key demo --ss-server 127.0.0.1:8388 --ss-method chacha20-ietf-poly1305 --ss-password your-password
```

## 作为 Go 包直接调用客户端

如果你要在第三方 Go 程序里直接嵌入客户端，不要导入 `internal/client`，请改用公开包 `github.com/pizixi/gpipe/client`。

完整示例可参考：

- `examples/third_party_go_client`：独立第三方模块示例，包含自己的 `go.mod`，并支持通过源码变量启用 Shadowsocks 出站
- `examples/client_direct_demo.go`：仓库内直接运行的完整示例，包含可选 Shadowsocks 出站拨号

最小示例：

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

  logger := log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)

  if err := gclient.RunContext(ctx, gclient.Options{
    Server: "tcp://127.0.0.1:8118",
    Key:    "demo",
    Logger: logger,
  }); err != nil {
    log.Fatal(err)
  }
}
```

如果需要通过 Shadowsocks 连接服务端，可以直接复用公开包里的拨号辅助：

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

## TLS 作用范围

`enable_tls` 只控制客户端和服务端之间的传输连接，不会自动把整条业务链路的每一段都变成 TLS。

以一条普通隧道为例，数据通常会经过三段：

1. 本地应用 `<->` 本地 `gpipe`
2. `gpipe client` `<->` `gpipe server`
3. 远端 `gpipe` `<->` 最终目标服务 `endpoint`

说明如下：

- 开启服务端配置 `enable_tls=true`，并在客户端使用 `--enable-tls` 后，第 `2` 段会使用 TLS
- 这包括隧道内承载的 `TCP / UDP / SOCKS5 / HTTP` 业务数据；这些数据在客户端和服务端之间传输时会被 TLS 保护
- 当前客户端 TLS 默认只做链路加密，不校验证书链和主机名，因此不需要再额外传 `--ca-cert`
- `--tls-server-name` 仍可用于发送指定的 TLS SNI
- 第 `1` 段和第 `3` 段不会因为 `enable_tls` 自动加密，它们是否加密取决于业务程序本身
- Web 管理端 `web_addr` 不受 `enable_tls` 控制，当前仍是独立的 HTTP 管理接口
- `quic://` 本身要求 TLS；`tcp://`、`ws://`、`kcp://` 在启用 `enable_tls` 后同样会对客户端和服务端之间的链路加密

例如：

- 本地 `iperf3` 连到本地 `gpipe`，这段不是 `enable_tls` 负责的
- 本地 `gpipe client` 到远端 `gpipe server`，这段会被 TLS 加密
- 远端 `gpipe` 再去连接 `127.0.0.1:5201`，这段仍然是普通 TCP/UDP，除非目标服务自己就是 TLS 服务

## 本地证书

仓库内置的 PowerShell 脚本会默认生成适合本机调试的证书：

- DNS SAN：`localhost`
- IP SAN：`127.0.0.1`、`::1`

生成命令：

```powershell
.\generate-certificate.ps1 -Force
```

生成后文件位于 `.\certs\` 目录：

- `.\certs\cert.pem`
- `.\certs\server.key.pem`
- `.\certs\root-ca.pem`
- `.\certs\root-ca.key.pem`

生成后：

- 服务端使用 `.\certs\cert.pem` 和 `.\certs\server.key.pem`
- 客户端启用 TLS 时不需要再传 `--ca-cert`
- 如果你希望在本地开发时覆盖内置页面，可以把前端文件放在 `web_base_dir` 指向的目录中；既支持传统单文件 `index.html`，也支持当前仓库使用的 `templates/` 拆分结构

本地测试示例：

```powershell
.\bin\gpipe-server.exe -config-file .\gpipe.json
.\bin\gpipe-client.exe run --server tcp://127.0.0.1:8118 --key demo --enable-tls
```

## 安装系统服务

客户端支持安装为系统服务，当前通过 `github.com/kardianos/service` 实现，支持 Windows 和 Linux。

Windows / Linux 安装：

```powershell
.\bin\gpipe-client.exe install --server tcp://127.0.0.1:8118 --key demo
```

卸载服务：

```powershell
.\bin\gpipe-client.exe uninstall
```

说明：

- `run-service` 子命令保留给服务管理器调用，通常不需要手工执行
- 新安装的服务不再附带 `--ca-cert` 参数
- Windows 下安装和启动服务通常需要管理员权限
- Linux 下通常需要 `systemd` 环境和足够权限

## 本地验证

仓库内置了最小链路验证脚本：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke.ps1
```

该脚本会完成以下动作：

- 生成临时 `gpipe.json`
- 启动 Go 服务端
- 校验在 `web_base_dir=""` 时首页和静态资源会返回内置 `frontend/dist` 内容
- 通过 Web API 创建测试用户
- 启动 Go 客户端并验证登录成功

## 备注

- 服务端数据库驱动为纯 Go 实现，不依赖 CGO

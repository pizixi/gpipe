# third_party_go_client

这个目录模拟“第三方 Go 程序如何直接复用 `gpipe/client` 包”。

## 运行

```powershell
cd examples\third_party_go_client
go run .
```

如果要通过 Shadowsocks 出站连接服务端，请直接修改 `main.go` 里的变量：

```go
var (
	server = "ws://127.0.0.1:8119"
	key    = "demo"

	ssServer   = "127.0.0.1:8388"
	ssMethod   = "chacha20-ietf-poly1305"
	ssPassword = "your-password"
)
```

## 说明

- 当前 `go.mod` 里的 `replace github.com/pizixi/gpipe => ../..` 只是为了在本仓库里直接运行这个示例。
- 如果你把这份代码拷到真实第三方项目里，请删除 `replace`，并把 `require github.com/pizixi/gpipe v0.0.0` 改成正式版本号。
- `ssServer`、`ssMethod`、`ssPassword` 这三个变量需要同时设置；只设置其中一部分会直接报错。
- Shadowsocks 自定义拨号当前只适用于 `tcp://` 和 `ws://` 服务端地址；`quic://`、`kcp://` 不走这个拨号器。
- 示例内部实际使用的是 `gclient.NewShadowsocksDialFunc(...)`；如果你要更细粒度控制，也可以改成 `gclient.NewSSDialer(...)`。

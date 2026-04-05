module example.com/gpipe-third-party-client-demo

go 1.26.1

require github.com/pizixi/gpipe v0.0.0

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/jedisct1/go-aes-siv v1.0.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.6 // indirect
	github.com/klauspost/reedsolomon v1.12.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/quic-go/quic-go v0.54.0 // indirect
	github.com/riobard/go-bloom v0.0.0-20200614022211-cdc8013cb5b3 // indirect
	github.com/shadowsocks/go-shadowsocks2 v0.1.5 // indirect
	github.com/tjfoc/gmsm v1.4.1 // indirect
	github.com/xtaci/kcp-go/v5 v5.6.72 // indirect
	go.uber.org/mock v0.5.0 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	golang.org/x/tools v0.42.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
)

// 便于在当前仓库里直接运行；真实第三方项目中请删除该 replace，
// 并把上面的 require 改成你要依赖的正式版本号。
replace github.com/pizixi/gpipe => ../..

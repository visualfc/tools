module github.com/goplus/tools/goxls

go 1.18

require golang.org/x/tools/gopls v0.13.2

require (
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/goplus/gop v1.1.7 // indirect
	github.com/qiniu/x v1.11.9 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20221212164502-fae10dda9338 // indirect
	golang.org/x/mod v0.13.0 // indirect
	golang.org/x/sync v0.4.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/telemetry v0.0.0-20231003223302-0168ef4ebbd3 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/tools v0.13.1-0.20230920233436-f9b8da7b22be // indirect
	golang.org/x/vuln v1.0.1 // indirect
	honnef.co/go/tools v0.4.5 // indirect
	mvdan.cc/gofumpt v0.4.0 // indirect
	mvdan.cc/xurls/v2 v2.4.0 // indirect
)

replace (
	golang.org/x/tools => ../
	golang.org/x/tools/gopls => ../gopls
)

module github.com/clktmr/n64

go 1.24

require (
	github.com/embeddedgo/display v1.1.0
	github.com/embeddedgo/fs v0.1.0
	github.com/ericpauley/go-quantize v0.0.0-20200331213906-ae555eb2afa4
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/sigurn/crc8 v0.0.0-20220107193325-2243fe600f9f
	golang.org/x/image v0.13.0
	golang.org/x/text v0.13.0
	rsc.io/rsc v0.0.0-20180427141835-fc6202590229
)

require (
	golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b // indirect
	golang.org/x/mod v0.25.0 // indirect
	golang.org/x/sync v0.15.0 // indirect
	golang.org/x/tools v0.34.0 // indirect
)

tool (
	github.com/clktmr/n64/tools/n64go
	golang.org/x/exp/cmd/gorelease
)

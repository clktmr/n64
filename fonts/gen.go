package fonts

//go:generate go tool n64go font basicfont
//go:generate go tool n64go font gomono
//go:generate go tool n64go font -start 0x2000 -end 0x26ff gomono
//go:generate go tool n64go font goregular
//go:generate go tool n64go font -start 0x2000 -end 0x26ff goregular

// Package fixed provides fixed-point arithmetic types used by the RCP.
package fixed

import "strconv"

//go:generate go run mkfixed.go UInt8 uint8
//go:generate go run mkfixed.go UInt14_2 uint16
//go:generate go run mkfixed.go Int11_5 int16
//go:generate go run mkfixed.go Int6_10 int16

func asString(x, frac int64, ip, fp uint8) string {
	var mask = int64(1<<frac - 1)

	s := make([]byte, 0, 2+ip+fp)
	if x < 0 {
		s = append(s, '-')
		x = -x
	}

	s = strconv.AppendUint(s, uint64(x>>frac), 10)
	s = append(s, ':')
	ss := strconv.AppendUint(s, uint64(x&mask), 10)

	fracstart := len(s)
	fracend := len(ss)
	padlen := int(fp) - (fracend - fracstart)
	s = s[:len(s)+int(fp)]
	copy(s[fracstart+padlen:fracend+padlen], s[fracstart:fracend])
	for i := range s[fracstart : fracstart+padlen] {
		s[fracstart+i] = '0'
	}

	return string(s)
}

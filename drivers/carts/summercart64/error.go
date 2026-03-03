package summercart64

import "strconv"

var configErrors = [...]string{
	0: "ok",
	1: "unknown command",
	2: "invalid argument",
	3: "invalid address",
	4: "invalid id",
}

var sdcardErrors = [...]string{
	0:  "ok",
	1:  "no card in slot",
	2:  "not initialized",
	3:  "invalid argument",
	4:  "invalid address",
	5:  "invalid operation",
	30: "locked",
}

type Error uint32

func (result Error) Error() string {
	errType := result >> 24
	errValue := result & 0xffffff
	switch errType {
	case 1: // config
		if int(errValue) < len(configErrors) {
			str := configErrors[errValue]
			if str != "" {
				return str
			}
		}

		return "config error " + strconv.Itoa(int(errValue))
	case 2: // sdcard
		if int(errValue) < len(sdcardErrors) {
			str := sdcardErrors[errValue]
			if str != "" {
				return str
			}
		}
		return "sdcard error " + strconv.Itoa(int(errValue))
	}

	return "sc64 error " + strconv.Itoa(int(errType)) + " " + strconv.Itoa(int(errValue))
}

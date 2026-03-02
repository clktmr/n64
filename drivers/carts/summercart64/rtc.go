package summercart64

import (
	"time"
)

func bcd2int(v uint32) int {
	return (int(v)&0xf0>>4)*10 + (int(v) & 0x0f)
}

func bcd(v int) uint32 {
	return uint32(((v/10)&0x0f)<<4 | ((v % 10) & 0x0f))
}

func (v *Cart) Time() (time.Time, error) {
	time0, time1, err := execCommand(cmdTimeGet, 0, 0)
	if err != nil {
		return time.Time{}, err
	}
	second := bcd2int(time0 >> 0 & 0xff)
	minute := bcd2int(time0 >> 8 & 0xff)
	hour := bcd2int(time0 >> 16 & 0xff)
	day := bcd2int(time1 >> 0 & 0xff)
	month := time.Month(bcd2int(time1 >> 8 & 0xff))
	year := bcd2int(time1>>16&0xff) + bcd2int(time1>>24&0xff)*100 + 1900
	return time.Date(year, month, day, hour, minute, second, 0, time.Local), nil
}

func (v *Cart) SetTime(t time.Time) error {
	time0 := bcd(t.Second()) | bcd(t.Minute())<<8 | bcd(t.Hour())<<16 | bcd(int(t.Weekday()))<<24
	time1 := bcd(t.Day()) | bcd(int(t.Month()))<<8 | bcd(t.Year()%100)<<16 | bcd((t.Year()-1900)/100)<<24
	_, _, err := execCommand(cmdTimeSet, time0, time1)
	if err != nil {
		return err
	}
	return nil
}

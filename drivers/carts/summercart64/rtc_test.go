package summercart64_test

import (
	"testing"
	"time"
)

func TestRTC(t *testing.T) {
	sc64 := mustSC64(t)

	nowrtc, err := sc64.Time()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(nowrtc)

	birthday := time.Date(1987, time.July, 21, 0, 0, 0, 0, time.Local)
	err = sc64.SetTime(birthday)
	if err != nil {
		t.Fatal(err)
	}

	testtime, err := sc64.Time()
	if err != nil {
		t.Fatal(err)
	}
	if !testtime.Truncate(time.Hour * 24).Equal(birthday) {
		t.Fatalf("expected %v, got %v", birthday, testtime.Truncate(time.Hour*24))
	}

	// restore time
	err = sc64.SetTime(nowrtc)
	if err != nil {
		t.Fatal(err)
	}
}

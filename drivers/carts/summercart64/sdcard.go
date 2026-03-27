package summercart64

import "io"

const sdSectorSize = 512
const sdSectorMask = sdSectorSize - 1

const (
	sdOpDeinit uint32 = iota
	sdOpInit
	sdOpStatus
	sdOpInfo
	sdOpByteSwapOn
	sdOpByteSwapOff
)

type SDCardStatus uint32

const (
	sdStatusInserted SDCardStatus = 1 << iota
	sdStatusInitialized
	sdStatusSectorAddressed
	sdStatus50MHzClock
	sdStatusByteSwap
)

func (s SDCardStatus) Inserted() bool {
	return s&sdStatusInserted != 0
}

func (s SDCardStatus) String() (r string) {
	if !s.Inserted() {
		return "not inserted"
	}
	r += "inserted"
	if s&sdStatusInitialized == 0 {
		return
	}
	r += ", initialized"
	if s&sdStatusSectorAddressed != 0 {
		r += ", sector addressed"
	} else {
		r += ", byte addressed"
	}
	if s&sdStatus50MHzClock != 0 {
		r += ", 50MHz"
	} else {
		r += ", 25MHz"
	}
	if s&sdStatusByteSwap != 0 {
		r += ", byte swap"
	}

	return
}

type SDCard struct{}

func (_ *Cart) SDCard() *SDCard {
	return &SDCard{}
}

func (_ *SDCard) Init() error   { _, _, err := execCommand(cmdSDCardOp, 0, sdOpInit); return err }
func (_ *SDCard) Deinit() error { _, _, err := execCommand(cmdSDCardOp, 0, sdOpDeinit); return err }
func (_ *SDCard) Status() (SDCardStatus, error) {
	_, status, err := execCommand(cmdSDCardOp, 0, sdOpStatus)
	return SDCardStatus(status), err
}

func (v *SDCard) ReadAt(p []byte, off int64) (n int, err error) {
	startSector := uint32(off / sdSectorSize)
	_, _, err = execCommand(cmdSDSectorSet, startSector, 0)
	if err != nil {
		return
	}

	startOffset := off & sdSectorMask
	for n < len(p) {
		sectorCnt := min(((len(p)-n)>>9)+1, sdcardBuf.Size()/sdSectorSize)
		_, _, err = execCommand(cmdSDRead, uint32(sdcardBuf.Addr()), uint32(sectorCnt))
		if err != nil {
			return
		}

		stop := min(n+(sdSectorSize*sectorCnt)-int(startOffset), len(p))
		var nn int
		nn, err = sdcardBuf.ReadAt(p[n:stop], startOffset)
		n += nn
		startOffset = 0 // reset, only for first iteration needed
		if err == io.EOF {
			err = nil
		} else if err != nil {
			return
		}

		off += int64(nn)
	}

	return
}

func (v *SDCard) WriteAt(p []byte, off int64) (n int, err error) {
	panic("not implemented")
}

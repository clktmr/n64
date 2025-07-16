package mixer_test

import (
	"bytes"
	"embed"
	"encoding/binary"
	"io"
	"math"
	"slices"
	"sync"
	"testing"

	"github.com/clktmr/n64/drivers/cartfs"
	"github.com/clktmr/n64/drivers/controller"
	"github.com/clktmr/n64/drivers/rspq"
	"github.com/clktmr/n64/drivers/rspq/mixer"
	"github.com/clktmr/n64/rcp/audio"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/serial/joybus"
)

var (
	//go:embed sfx_alarm_loop3.pcm_s16be
	//go:embed sfx_wpn_cannon2.pcm_s16be
	//go:embed sfx_wpn_machinegun_loop1.pcm_s16be
	_testdata embed.FS
	testdata  cartfs.FS = cartfs.Embed(_testdata)
)

func TestResampling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	rspq.Reset()
	mixer.Init()

	audio.Start(48000)
	mixer.SetSampleRate(48000)
	sfxAlarm, _ := testdata.Open("sfx_alarm_loop3.pcm_s16be")
	sfxCannon, _ := testdata.Open("sfx_wpn_cannon2.pcm_s16be")
	sfxMachinegun, _ := testdata.Open("sfx_wpn_machinegun_loop1.pcm_s16be")
	sourceAlarm := mixer.NewSource(mixer.Loop(sfxAlarm.(io.ReadSeeker)), 16000)
	sourceCannon := mixer.NewSource(mixer.Loop(sfxCannon.(io.ReadSeeker)), 44100)
	sourceMachinegun := mixer.NewSource(mixer.Loop(sfxMachinegun.(io.ReadSeeker)), 8000)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() { audio.Buffer.ReadFrom(mixer.Output); wg.Done() }()

	t.Log("Hold C buttons to enable audio sources:")
	t.Log(joybus.ButtonCLeft, "= alarm,", joybus.ButtonCDown, "= explosion,", joybus.ButtonCRight, "= machinegun")
	t.Log("Press any other button otherwise.")
	var states [4]controller.Controller
	for {
		controller.Poll(&states)
		switch {
		case states[0].Pressed()&joybus.ButtonCLeft != 0:
			mixer.SetSource(0, sourceAlarm)
		case states[0].Released()&joybus.ButtonCLeft != 0:
			mixer.SetSource(0, nil)
		case states[0].Pressed()&joybus.ButtonCDown != 0:
			mixer.SetSource(1, sourceCannon)
		case states[0].Released()&joybus.ButtonCDown != 0:
			mixer.SetSource(1, nil)
		case states[0].Pressed()&joybus.ButtonCRight != 0:
			mixer.SetSource(2, sourceMachinegun)
		case states[0].Released()&joybus.ButtonCRight != 0:
			mixer.SetSource(2, nil)
		case states[0].Pressed()&joybus.ButtonReset != 0:
			goto end
		}
		s := sourceMachinegun
		s.SetVolume(1.0, (float32(states[0].X())/85.0/2.0)+0.5)
		pitch := 8000 * ((float32(states[0].Y()) / 85.0) + 1.0)
		s.SetSampleRate(uint(min(max(0, pitch), 128000)))
	}
end:
	audio.Stop()
	wg.Wait()
}

func TestMixing(t *testing.T) {
	rspq.Reset()
	mixer.Init()

	var sinus, cosinus [32]int16
	var expected, result [32]int16
	for i := range 32 {
		sinus[i] = int16(math.Sin(2*math.Pi*float64(i)/16) * float64(math.MaxInt16/2))
		cosinus[i] = int16(math.Cos(2*math.Pi*float64(i)/16) * float64(math.MaxInt16/2))
	}
	for i := range 16 {
		expected[i<<1] = (sinus[i] + cosinus[i]) >> 1   // left channel
		expected[i<<1+1] = (sinus[i] + cosinus[i]) >> 1 // right channel
	}

	sinusBuf := cpu.MakePaddedSliceAligned[byte](64, 16)
	_, err := binary.Encode(sinusBuf, binary.BigEndian, sinus)
	if err != nil {
		t.Fatal(err)
	}
	cosinusBuf := cpu.MakePaddedSliceAligned[byte](64, 16)
	_, err = binary.Encode(cosinusBuf, binary.BigEndian, cosinus)
	if err != nil {
		t.Fatal(err)
	}

	cpu.WritebackSlice(sinusBuf)
	cpu.WritebackSlice(cosinusBuf)

	mixer.SetSampleRate(8000)

	mixer.SetSource(0, mixer.NewSource(mixer.Loop(bytes.NewReader(sinusBuf)), 8000))
	mixer.SetSource(1, mixer.NewSource(mixer.Loop(bytes.NewReader(cosinusBuf)), 8000))

	resultBuf := cpu.MakePaddedSliceAligned[byte](8192, 16)
	_, err = io.ReadFull(mixer.Output, resultBuf)
	if err != nil {
		t.Fatal(err)
	}

	resultBuf = resultBuf[len(resultBuf)-64:] // read from end to minimize one-tap filter effect
	_, err = binary.Decode(resultBuf, binary.BigEndian, &result)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.EqualFunc(result[:], expected[:], func(a, b int16) bool {
		// allow some error, not sure what causes them
		diff := int(a) - int(b)
		return max(diff, -diff) <= 8
	}) {
		t.Error("expected", expected)
		t.Error("got", result)
	}
}

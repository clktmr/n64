package pakfs

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

const rcd = '�' // decoding replacement character

var decode = [...]rune{
	'\u0000', rcd, rcd, rcd, rcd, rcd, rcd, rcd, rcd, rcd, rcd, rcd, rcd, rcd, rcd, ' ',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'A', 'B', 'C', 'D', 'E', 'F',
	'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V',
	'W', 'X', 'Y', 'Z', '!', '"', '#', '\'', '*', '+', ',', '-', '.', '/', ':', '=',
	'?', '@', '。', '゛', '゜', 'ァ', 'ィ', 'ゥ', 'ェ', 'ォ', 'ッ', 'ャ', 'ュ', 'ョ', 'ヲ', 'ン',
	'ア', 'イ', 'ウ', 'エ', 'オ', 'カ', 'キ', 'ク', 'ケ', 'コ', 'サ', 'シ', 'ス', 'セ', 'ソ', 'タ',
	'チ', 'ツ', 'テ', 'ト', 'ナ', 'ニ', 'ヌ', 'ネ', 'ノ', 'ハ', 'ヒ', 'フ', 'ヘ', 'ホ', 'マ', 'ミ',
	'ム', 'メ', 'モ', 'ヤ', 'ユ', 'ヨ', 'ラ', 'リ', 'ル', 'レ', 'ロ', 'ワ', 'ガ', 'ギ', 'グ', 'ゲ',
	'ゴ', 'ザ', 'ジ', 'ズ', 'ゼ', 'ゾ', 'ダ', 'ヂ', 'ヅ', 'デ', 'ド', 'バ', 'ビ', 'ブ', 'ベ', 'ボ',
	'パ', 'ピ', 'プ', 'ペ', 'ポ',
}

const rce = 64 // encoding replacement character

var encode = map[rune]byte{
	'\u0000': 0, ' ': 15,
	'0': 16, '1': 17, '2': 18, '3': 19, '4': 20, '5': 21, '6': 22, '7': 23, '8': 24, '9': 25, 'A': 26, 'B': 27, 'C': 28, 'D': 29, 'E': 30, 'F': 31,
	'G': 32, 'H': 33, 'I': 34, 'J': 35, 'K': 36, 'L': 37, 'M': 38, 'N': 39, 'O': 40, 'P': 41, 'Q': 42, 'R': 43, 'S': 44, 'T': 45, 'U': 46, 'V': 47,
	'W': 48, 'X': 49, 'Y': 50, 'Z': 51, '!': 52, '"': 53, '#': 54, '\'': 55, '*': 56, '+': 57, ',': 58, '-': 59, '.': 60, '/': 61, ':': 62, '=': 63,
	'?': 64, '@': 65, '。': 66, '゛': 67, '゜': 68, 'ァ': 69, 'ィ': 70, 'ゥ': 71, 'ェ': 72, 'ォ': 73, 'ッ': 74, 'ャ': 75, 'ュ': 76, 'ョ': 77, 'ヲ': 78, 'ン': 79,
	'ア': 80, 'イ': 81, 'ウ': 82, 'エ': 83, 'オ': 84, 'カ': 85, 'キ': 86, 'ク': 87, 'ケ': 88, 'コ': 89, 'サ': 90, 'シ': 91, 'ス': 92, 'セ': 93, 'ソ': 94, 'タ': 95,
	'チ': 96, 'ツ': 97, 'テ': 98, 'ト': 99, 'ナ': 100, 'ニ': 101, 'ヌ': 102, 'ネ': 103, 'ノ': 104, 'ハ': 105, 'ヒ': 106, 'フ': 107, 'ヘ': 108, 'ホ': 109, 'マ': 110, 'ミ': 111,
	'ム': 112, 'メ': 113, 'モ': 114, 'ヤ': 115, 'ユ': 116, 'ヨ': 117, 'ラ': 118, 'リ': 119, 'ル': 120, 'レ': 121, 'ロ': 122, 'ワ': 123, 'ガ': 124, 'ギ': 125, 'グ': 126, 'ゲ': 127,
	'ゴ': 128, 'ザ': 129, 'ジ': 130, 'ズ': 131, 'ゼ': 132, 'ゾ': 133, 'ダ': 134, 'ヂ': 135, 'ヅ': 136, 'デ': 137, 'ド': 138, 'バ': 139, 'ビ': 140, 'ブ': 141, 'ベ': 142, 'ボ': 143,
	'パ': 144, 'ピ': 145, 'プ': 146, 'ペ': 147, 'ポ': 148,
}

type charmap struct{}

var N64FontCode encoding.Encoding = &charmap{}

func (m *charmap) NewDecoder() *encoding.Decoder {
	return &encoding.Decoder{Transformer: &decoder{}}
}

func (m *charmap) NewEncoder() *encoding.Encoder {
	return &encoding.Encoder{Transformer: &encoder{}}
}

type decoder struct{}

func (d *decoder) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for _, c := range src {
		nSrc += 1
		var r rune = rcd
		if c < byte(len(decode)) {
			r = decode[c]
		}
		rlen := utf8.RuneLen(r) // r is always valid
		if rlen > len(dst)-nDst {
			err = transform.ErrShortDst
			break
		}

		nDst += utf8.EncodeRune(dst[nDst:], decode[c])
	}
	return
}

func (d *decoder) Reset() {}

type encoder struct{}

func (d *encoder) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	if atEOF == false {
		// TODO
		err = errors.New("not implemented")
		return
	}
	for {
		r, size := utf8.DecodeRune(src[nSrc:])
		if size < 1 {
			break
		}
		nSrc += size
		// TODO handle incomplete rune with atEOF == false
		if nDst >= len(dst) {
			err = transform.ErrShortDst
			// FIXME nSrc -= size ?
			break
		}
		if c, ok := encode[r]; ok {
			dst[nDst] = c
		} else {
			dst[nDst] = rce
		}
		nDst += 1
	}
	return
}

func (d *encoder) Reset() {}

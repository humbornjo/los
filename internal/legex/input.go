package legex

import (
	"unicode/utf8"
)

type inputBytes struct {
	str []byte
}

func (i *inputBytes) step(pos int) (rune, int) {
	if pos < len(i.str) {
		c := i.str[pos]
		if c < utf8.RuneSelf {
			return rune(c), 1
		}
		return utf8.DecodeRune(i.str[pos:])
	}
	return endOfText, 0
}

func (i *inputBytes) context(pos int) lazyFlag {
	r1, r2 := endOfText, endOfText
	// 0 < pos && pos <= len(i.str)
	if uint(pos-1) < uint(len(i.str)) {
		r1 = rune(i.str[pos-1])
		if r1 >= utf8.RuneSelf {
			r1, _ = utf8.DecodeLastRune(i.str[:pos])
		}
	}
	// 0 <= pos && pos < len(i.str)
	if uint(pos) < uint(len(i.str)) {
		r2 = rune(i.str[pos])
		if r2 >= utf8.RuneSelf {
			r2, _ = utf8.DecodeRune(i.str[pos:])
		}
	}
	return newLazyFlag(r1, r2)
}

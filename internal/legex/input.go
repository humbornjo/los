package legex

import (
	"bytes"
	"unicode/utf8"
)

// input abstracts different representations of the input text. It provides
// one-character lookahead.
type input interface {
	canCheckPrefix() bool      // can we look ahead without losing info?
	hasPrefix(re *Regexp) bool // check if the input starts with the literal prefix

	length() int
	next(pos int) (r rune, width int) // advance one rune
	context(pos int) lazyFlag         // generate match flag for the current position
	prologue(re *Regexp, index int, offset int) (idx int, off int)
}

// inputBytes scans a byte slice.
type inputBytes struct {
	str []byte
}

func (i *inputBytes) canCheckPrefix() bool {
	return true
}

func (i *inputBytes) hasPrefix(re *Regexp) bool {
	return bytes.HasPrefix(i.str, re.prefixBytes)
}

func (i *inputBytes) length() int {
	return len(i.str)
}

func (i *inputBytes) next(pos int) (rune, int) {
	if pos < len(i.str) {
		c := i.str[pos] // i.str[pos]
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

func (i *inputBytes) prologue(re *Regexp, index int, offset int) (int, int) {
	n0, n1 := len(re.prefix), len(i.str)
	i0, i1 := offset, index
	for i0 < n0 && i1+i0 < n1 {
		if re.prefix[i0] != i.str[i1+i0] {
			i0, i1 = 0, i1+1
			continue
		}
		i0++
	}
	return i1, i0
}

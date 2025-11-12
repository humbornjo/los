package los

type pattern interface {
	// match advance the match index and offset to release the
	// unmatched string in buffer ASAP.
	match(index int, offset int, s []byte) (newIndex int, newOffset int, ok bool)
}

// Implemented with Knuth-Morris-Pratt algorithm for forward
// search.
type defaultPattern struct {
	lps    []int
	length int
	source string
}

var _ pattern = (*defaultPattern)(nil)

func newDefaultPattern(source string) *defaultPattern {
	computeLpsArray := func(pattern string) []int {
		n := len(pattern)
		array := make([]int, n)
		for i, j := 1, 0; i < n; {
			if pattern[i] == pattern[j] {
				j++
				array[i], i = j, i+1
			} else {
				if j != 0 {
					j = array[j-1]
				} else {
					array[i], i = 0, i+1
				}
			}
		}
		return array
	}
	return &defaultPattern{computeLpsArray(source), len(source), source}
}

func (pat *defaultPattern) match(index int, offset int, buffer []byte) (int, int, bool) {
	if offset == pat.length {
		return index, offset, true
	}
	n, m := len(buffer), pat.length
	i, j := index+offset, offset // start match index with offset
	for i < n {
		if buffer[i] == pat.source[j] {
			i, j = i+1, j+1
			if j == m {
				return i - j, j, true
			}
		} else {
			if j != 0 {
				j = pat.lps[j-1]
			} else {
				i++
			}
		}
	}
	return i - j, j, false
}

// Implemented with regular expression VM for forward search.
//
// - https://swtch.com/~rsc/regexp/regexp2.html
type regexPattern struct {
}

var _ pattern = (*regexPattern)(nil)

func (pat *regexPattern) match(index int, offset int, buffer []byte) (int, int, bool) {
	return 0, 0, false
}

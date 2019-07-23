package oscript

import "bytes"

// Compact appends to dst the oscript-encoded src with
// insignificant space characters elided.
func Compact(dst *bytes.Buffer, src []byte) error {
	return compact(dst, src)
}

func compact(dst *bytes.Buffer, src []byte) error {
	origLen := dst.Len()
	var scan scanner
	scan.reset()
	start := 0
	for i, c := range src {
		// Convert U+2028 and U+2029 (E2 80 A8 and E2 80 A9).
		if c == 0xE2 && i+2 < len(src) && src[i+1] == 0x80 && src[i+2]&^1 == 0xA8 {
			if start < i {
				dst.Write(src[start:i])
			}
			dst.WriteString(`\u202`)
			dst.WriteByte(hex[src[i+2]&0xF])
			start = i + 3
		}
		v := scan.step(&scan, c)
		if v >= scanSkipSpace {
			if v == scanError {
				break
			}

			if start < i {
				dst.Write(src[start:i])
			}
			start = i + 1
		}
	}

	if scan.eof() == scanError {
		dst.Truncate(origLen)
		return scan.err
	}

	if start < len(src) {
		dst.Write(src[start:])
	}
	return nil
}

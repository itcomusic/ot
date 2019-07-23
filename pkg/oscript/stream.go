package oscript

import (
	"bytes"
	"io"
)

// A Decoder reads and decodes Oscript values from an input stream.
type Decoder struct {
	r       io.Reader
	buf     []byte
	d       decodeState
	scanp   int   // start of unread data in buf
	scanned int64 // amount of data already scanned
	scan    scanner
	err     error

	tokenState int
	tokenStack []int
}

// NewDecoder returns a new decoder that reads from r.
//
// The decoder introduces its own buffering and may
// read data from r beyond the oscript values requested.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// DisallowUnknownFields causes the Decoder to return an error when the destination
// is a struct and the input contains object keys which do not match any
// non-ignored, exported fields in the destination.
func (dec *Decoder) DisallowUnknownFields() { dec.d.disallowUnknownFields = true }

// Decode reads the next Oscript-encoded value from its
// input and stores it in the value pointed to by v.
//
// See the documentation for Unmarshal for details about
// the conversion of Oscript into a Go value.
func (dec *Decoder) Decode(v interface{}) error {
	if dec.err != nil {
		return dec.err
	}

	if err := dec.tokenPrepareForDecode(); err != nil {
		return err
	}

	if !dec.tokenValueAllowed() {
		return &SyntaxError{msg: "not at beginning of value", Offset: dec.offset()}
	}

	// Read whole value into buffer.
	n, err := dec.readValue()
	if err != nil {
		return err
	}
	dec.d.init(dec.buf[dec.scanp : dec.scanp+n])
	dec.scanp += n

	// Don't save err from unmarshal into dec.err:
	// the connection is still usable since we read a complete oscript
	// object from it before the error happened.
	err = dec.d.unmarshal(v)
	// fixup token streaming state
	dec.tokenValueEnd()
	return err
}

// Buffered returns a reader of the data remaining in the Decoder's
// buffer. The reader is valid until the next call to Decode.
func (dec *Decoder) Buffered() io.Reader {
	return bytes.NewReader(dec.buf[dec.scanp:])
}

// readValue reads a Oscript value into dec.buf.
// It returns the length of the encoding.
func (dec *Decoder) readValue() (int, error) {
	dec.scan.reset()

	scanp := dec.scanp
	var err error
Input:
	for {
		// Look in the buffer for a new value.
		for i, c := range dec.buf[scanp:] {
			dec.scan.bytes++
			v := dec.scan.step(&dec.scan, c)
			if v == scanEnd {
				scanp += i
				break Input
			}
			// scanEnd is delayed one byte.
			// We might block trying to get that byte from src,
			// so instead invent a space byte.
			if (v == scanEndObject || v == scanEndArray) && dec.scan.step(&dec.scan, ' ') == scanEnd {
				scanp += i + 1
				break Input
			}

			if v == scanError {
				dec.err = dec.scan.err
				return 0, dec.scan.err
			}
		}
		scanp = len(dec.buf)

		// Did the last read have an error?
		// Delayed until now to allow buffer scan.
		if err != nil {
			if err == io.EOF {
				if dec.scan.step(&dec.scan, ' ') == scanEnd {
					break Input
				}

				if nonSpace(dec.buf) {
					err = io.ErrUnexpectedEOF
				}
			}
			dec.err = err
			return 0, err
		}
		n := scanp - dec.scanp
		err = dec.refill()
		scanp = dec.scanp + n
	}
	return scanp - dec.scanp, nil
}

func (dec *Decoder) tokenValueAllowed() bool {
	switch dec.tokenState {
	case tokenTopValue, tokenArrayStart, tokenArrayValue, tokenObjectValue:
		return true
	}
	return false
}

func (dec *Decoder) tokenValueEnd() {
	switch dec.tokenState {
	case tokenArrayStart, tokenArrayValue:
		dec.tokenState = tokenArrayComma
	case tokenObjectValue:
		dec.tokenState = tokenObjectComma
	}
}

// A Token holds a value of one of these types:
//
//	Delim, for the four Oscript delimiters A > { }
//	bool, for Oscript booleans
//	number, for Oscript integers
//	float64, for Oscript reals
//	string, for Oscript string literals
//	nil, for Oscript ?
//
type Token interface{}

const (
	tokenTopValue = iota
	tokenArrayStart
	tokenArrayValue
	tokenArrayComma
	tokenObjectStart
	tokenObjectBeginKey
	tokenObjectKey
	tokenObjectEqually
	tokenObjectValue
	tokenObjectComma
)

// advance tokenstate from a separator state to a value state
func (dec *Decoder) tokenPrepareForDecode() error {
	// Note: Not calling peek before switch, to avoid
	// putting peek into the standard Decode path.
	// peek is only called when using the Token API.
	switch dec.tokenState {
	case tokenArrayComma:
		c, err := dec.peek()
		if err != nil {
			return err
		}

		if c != ',' {
			return &SyntaxError{"expected comma after array element", dec.offset()}
		}
		dec.scanp++
		dec.tokenState = tokenArrayValue

	case tokenObjectEqually:
		c, err := dec.peek()
		if err != nil {
			return err
		}

		if c != '=' {
			return &SyntaxError{"expected equally after object key", dec.offset()}
		}
		dec.scanp++
		dec.tokenState = tokenObjectValue
	}
	return nil
}

// A Delim is a Oscript array or object delimiter, one of { } A or >.
type Delim rune

func (d Delim) String() string {
	return string(d)
}

// Token returns the next Oscript token in the input stream.
// At the end of the input stream, Token returns nil, io.EOF.
//
// Token guarantees that the delimiters A<1,? > { } it returns are
// properly nested and matched: if Token encounters an unexpected
// delimiter in the input, it will return an error.
//
// The input stream consists of basic Oscript values—bool, string,
// number, and undefined—along with delimiters A<1,? > { } of type Delim
// to mark the start and end of arrays and objects.
// Commas and equally are elided.
func (dec *Decoder) Token() (Token, error) {
	for {
		c, err := dec.peek()
		if err != nil {
			return nil, err
		}
		switch c {
		case '{':
			if !dec.tokenValueAllowed() {
				return dec.tokenError(c)
			}

			dec.scanp++
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
			dec.tokenState = tokenArrayStart
			return Delim('{'), nil

		case '}':
			if dec.tokenState != tokenArrayStart && dec.tokenState != tokenArrayComma {
				return dec.tokenError(c)
			}

			dec.scanp++
			dec.tokenState = dec.tokenStack[len(dec.tokenStack)-1]
			dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
			dec.tokenValueEnd()
			return Delim('}'), nil

		case 'A':
			if !dec.tokenValueAllowed() {
				return dec.tokenError(c)
			}

			dec.scanp++
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
			dec.tokenState = tokenObjectStart
			return Delim('A'), nil

		case '<':
			if dec.tokenState != tokenObjectStart {
				return dec.tokenError(c)
			}
			dec.scanp++
			continue

		case '>':
			if dec.tokenState != tokenObjectStart && dec.tokenState != tokenObjectComma {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenState = dec.tokenStack[len(dec.tokenStack)-1]
			dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
			dec.tokenValueEnd()
			return Delim('>'), nil

		case '=':
			if dec.tokenState != tokenObjectEqually {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenState = tokenObjectValue
			continue

		case ',':
			if dec.tokenState == tokenArrayComma {
				dec.scanp++
				dec.tokenState = tokenArrayValue
				continue
			}

			if dec.tokenState == tokenObjectComma {
				dec.scanp++
				dec.tokenState = tokenObjectKey
				continue
			}

			if dec.tokenState == tokenObjectStart {
				dec.scanp++
				continue
			}

			if dec.tokenState == tokenObjectBeginKey {
				dec.scanp++
				dec.tokenState = tokenObjectKey
				continue
			}
			return dec.tokenError(c)

		case '\'':
			if dec.tokenState == tokenObjectKey {
				var x string
				old := dec.tokenState
				dec.tokenState = tokenTopValue
				err := dec.Decode(&x)
				dec.tokenState = old
				if err != nil {
					return nil, err
				}
				dec.tokenState = tokenObjectEqually
				return x, nil
			}
			fallthrough

		default:
			switch c {
			case '1':
				if dec.tokenState == tokenObjectStart {
					dec.scanp++
					continue
				}
			case '?', 'N':
				// '?' can meet in A<1,?.
				// 'N' can meet in <A,N.
				// N may be meet in N other struct data, but now not released
				if dec.tokenState == tokenObjectStart {
					dec.scanp++
					dec.tokenState = tokenObjectBeginKey
					continue
				}
			}
			if !dec.tokenValueAllowed() {
				return dec.tokenError(c)
			}
			var x interface{}
			if err := dec.Decode(&x); err != nil {
				return nil, err
			}
			return x, nil
		}
	}
}

func (dec *Decoder) tokenError(c byte) (Token, error) {
	var context string
	switch dec.tokenState {
	case tokenTopValue:
		context = " looking for beginning of value"

	case tokenArrayStart, tokenArrayValue, tokenObjectValue:
		context = " looking for beginning of value"

	case tokenArrayComma:
		context = " after array element"

	case tokenObjectKey:
		context = " looking for beginning of object key string"

	case tokenObjectEqually:
		context = " after object key"

	case tokenObjectComma:
		context = " after object key:value pair"
	}
	return nil, &SyntaxError{"invalid character " + quoteChar(c) + " " + context, dec.offset()}
}

// More reports whether there is another element in the
// current array or object being parsed.
func (dec *Decoder) More() bool {
	c, err := dec.peek()
	return err == nil && c != '}' && c != '>'

}

func (dec *Decoder) offset() int64 {
	return dec.scanned + int64(dec.scanp)
}

func (dec *Decoder) peek() (byte, error) {
	var err error
	for {
		for i := dec.scanp; i < len(dec.buf); i++ {
			c := dec.buf[i]
			if isSpace(c) {
				continue
			}

			dec.scanp = i
			return c, nil
		}

		// buffer has been scanned, now report any error
		if err != nil {
			return 0, err
		}
		err = dec.refill()
	}
}

func (dec *Decoder) refill() error {
	// Make room to read more into the buffer.
	// First slide down data already consumed.
	if dec.scanp > 0 {
		dec.scanned += int64(dec.scanp)
		n := copy(dec.buf, dec.buf[dec.scanp:])
		dec.buf = dec.buf[:n]
		dec.scanp = 0
	}

	// Grow buffer if not large enough.
	const minRead = 512
	if cap(dec.buf)-len(dec.buf) < minRead {
		newBuf := make([]byte, len(dec.buf), 2*cap(dec.buf)+minRead)
		copy(newBuf, dec.buf)
		dec.buf = newBuf
	}
	// Read. Delay error for next iteration (after scan).
	n, err := dec.r.Read(dec.buf[len(dec.buf):cap(dec.buf)])
	dec.buf = dec.buf[0 : len(dec.buf)+n]
	return err
}

func nonSpace(b []byte) bool {
	for _, c := range b {
		if !isSpace(c) {
			return true
		}
	}
	return false
}

// An Encoder writes Oscript values to an output stream.
type Encoder struct {
	w   io.Writer
	err error

	enabledEncodeNL bool
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode writes the Oscript encoding of v to the stream,
// followed by a newline character.
//
// See the documentation for Marshal for details about the
// conversion of Go values to Oscript.
func (enc *Encoder) Encode(v interface{}) error {
	if enc.err != nil {
		return enc.err
	}
	e := newEncodeState()
	err := e.marshal(v)
	if err != nil {
		return err
	}
	// Terminate each value with a newline.
	// This makes the output look a little nicer
	// when debugging, and some kind of space
	// is required if the encoded value was a number,
	// so that the reader knows there aren't more
	// digits coming.
	if enc.enabledEncodeNL {
		e.WriteByte('\n')
	}
	b := e.Bytes()
	if _, err = enc.w.Write(b); err != nil {
		enc.err = err
	}
	encodeStatePool.Put(e)
	return err
}

// EnableEncodeNL enables newline after call Encode.
func (enc *Encoder) EnableEncodeNL() {
	enc.enabledEncodeNL = true
}

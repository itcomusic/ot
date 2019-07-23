package oscript

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"math"
	"math/big"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type T struct {
	X string
	Y int
	Z int `oscript:"-"`
}

type U struct {
	Alphabet string `oscript:"alpha"`
}

type V struct {
	F1 interface{}
	F2 int32
	F4 *VOuter
}

type VOuter struct {
	V V
}

// numAsInt64 are used to test unmarshaling.
var numAsInt64 = map[string]interface{}{
	"k1": int64(1),
	"k2": "s",
	"k3": []interface{}{int64(1), float64(2.0), float64(0.003)},
	"k4": map[string]interface{}{"kk1": "s", "kk2": int64(2)},
}

type tx struct {
	x int
}

type u8 uint8

// A type that can unmarshal itself.

type unmarshaler struct {
	T bool
}

func (u *unmarshaler) UnmarshalOscript(b []byte) error {
	*u = unmarshaler{true} // All we need to see that UnmarshalOscript is called.
	return nil
}

type ustruct struct {
	M unmarshaler
}

type unmarshalerText struct {
	A, B string
}

// needed for re-marshaling tests
func (u unmarshalerText) MarshalText() ([]byte, error) {
	return []byte(u.A + ":" + u.B), nil
}

func (u *unmarshalerText) UnmarshalText(b []byte) error {
	pos := bytes.IndexByte(b, ':')
	if pos == -1 {
		return errors.New("missing separator")
	}
	u.A, u.B = string(b[:pos]), string(b[pos+1:])
	return nil
}

// u8marshal is an integer type that can marshal/unmarshal itself.
type u8marshal uint8

func (u8 u8marshal) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("u%d", u8)), nil
}

var errMissingU8Prefix = errors.New("missing 'u' prefix")

func (u8 *u8marshal) UnmarshalText(b []byte) error {
	if !bytes.HasPrefix(b, []byte{'u'}) {
		return errMissingU8Prefix
	}

	n, err := strconv.Atoi(string(b[1:]))
	if err != nil {
		return err
	}
	*u8 = u8marshal(n)
	return nil
}

var (
	um0, um1 unmarshaler // target2 of unmarshaling
	ump      = &um1
	umtrue   = unmarshaler{true}
	umslice  = []unmarshaler{{true}}
	umslicep = new([]unmarshaler)
	umstruct = ustruct{unmarshaler{true}}

	ummapType = map[unmarshalerText]bool{}
	ummapXY   = map[unmarshalerText]bool{unmarshalerText{"x", "y"}: true}
)

type mapStringToStringData struct {
	Data map[string]string `oscript:"data"`
}

type unmarshalTest struct {
	indirect              bool
	in                    string
	ptr                   interface{}
	out                   interface{}
	err                   error
	golden                bool
	disallowUnknownFields bool
}

// Test data structures for anonymous fields.
type Point struct {
	Z int
}

type Top struct {
	Level0 int
	Embed0
	*Embed0a
	*Embed0b `oscript:"e,omitempty"` // treated as named
	Embed0c  `oscript:"-"`           // ignored
	Loop
	Embed0p // has Point with X, Y, used
	Embed0q // has Point with Z, used
	embed   // contains exported field
}

type Embed0 struct {
	Level1a int // overridden by Embed0a's Level1a with oscript tag
	Level1b int // used because Embed0a's Level1b is renamed
	Level1c int // used because Embed0a's Level1c is ignored
	Level1d int // annihilated by Embed0a's Level1d
	Level1e int `oscript:"x"` // annihilated by Embed0a.Level1e

}

type Embed0a struct {
	Level1a int `oscript:"Level1a,omitempty"`
	Level1b int `oscript:"LEVEL1B,omitempty"`
	Level1c int `oscript:"-"`
	Level1d int
	Level1f int `oscript:"x"` // annihilated by Embed0's Level1e

}

type Embed0b Embed0

type Embed0c Embed0

type Embed0p struct {
	image.Point
}

type Embed0q struct {
	Point
}

type embed struct {
	Q int
}

type Loop struct {
	Loop1 int `oscript:",omitempty"`
	Loop2 int `oscript:",omitempty"`
	*Loop
}

// From reflect test:
// The X in S6 and S7 annihilate, but they also block the X in S8.S9.
type S5 struct {
	S6
	S7
	S8
}

type S6 struct {
	X int
}

type S7 S6

type S8 struct {
	S9
}

type S9 struct {
	X int
	Y int
}

// From reflect test:
// The X in S11.S6 and S12.S6 annihilate, but they also block the X in S13.S8.S9.
type S10 struct {
	S11
	S12
	S13
}

type S11 struct {
	S6
}

type S12 struct {
	S6
}

type S13 struct {
	S8
}

type Ambig struct {
	// Given "hello", the first match should win.
	First  int `oscript:"HELLO"`
	Second int `oscript:"Hello"`
}

type unexportedWithMethods struct{}

func (unexportedWithMethods) F() {}

func sliceAddr(x []int) *[]int                 { return &x }
func mapAddr(x map[string]int) *map[string]int { return &x }

type byteWithMarshalOscript byte

func (b byteWithMarshalOscript) MarshalOscript() ([]byte, error) {
	return []byte(fmt.Sprintf(`'Z%.2x'`, byte(b))), nil
}

func (b *byteWithMarshalOscript) UnmarshalOscript(data []byte) error {
	if len(data) != 5 || data[0] != '\'' || data[1] != 'Z' || data[4] != '\'' {
		return fmt.Errorf("bad quoted string")
	}
	i, err := strconv.ParseInt(string(data[2:4]), 16, 8)

	if err != nil {
		return fmt.Errorf("bad hex")
	}
	*b = byteWithMarshalOscript(i)
	return nil
}

type byteWithPtrMarshalOscript byte

func (b *byteWithPtrMarshalOscript) MarshalOscript() ([]byte, error) {
	return byteWithMarshalOscript(*b).MarshalOscript()
}

func (b *byteWithPtrMarshalOscript) UnmarshalOscript(data []byte) error {
	return (*byteWithMarshalOscript)(b).UnmarshalOscript(data)
}

type intWithMarshalOscript int

func (b intWithMarshalOscript) MarshalOscript() ([]byte, error) {
	return []byte(fmt.Sprintf(`'Z%.2x'`, int(b))), nil
}

func (b *intWithMarshalOscript) UnmarshalOscript(data []byte) error {
	if len(data) != 5 || data[0] != '\'' || data[1] != 'Z' || data[4] != '\'' {
		return fmt.Errorf("bad quoted string")
	}
	i, err := strconv.ParseInt(string(data[2:4]), 16, 8)
	if err != nil {
		return fmt.Errorf("bad hex")
	}
	*b = intWithMarshalOscript(i)
	return nil
}

type intWithPtrMarshalOscript int

func (b *intWithPtrMarshalOscript) MarshalOscript() ([]byte, error) {
	return intWithMarshalOscript(*b).MarshalOscript()
}

func (b *intWithPtrMarshalOscript) UnmarshalOscript(data []byte) error {
	return (*intWithMarshalOscript)(b).UnmarshalOscript(data)
}

type intWithMarshalText int

func (b intWithMarshalText) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf(`Z%.2x`, int(b))), nil
}

func (b *intWithMarshalText) UnmarshalText(data []byte) error {
	if len(data) != 3 || data[0] != 'Z' {
		return fmt.Errorf("bad quoted string")
	}
	i, err := strconv.ParseInt(string(data[1:3]), 16, 8)
	if err != nil {
		return fmt.Errorf("bad hex")
	}
	*b = intWithMarshalText(i)
	return nil
}

var unmarshalTests = []unmarshalTest{
	// basic types
	{in: `true`, ptr: new(bool), out: true},
	{in: `1`, ptr: new(int), out: 1},
	{in: `L5`, ptr: new(int16), out: int16(5)},
	{in: `L-5`, ptr: new(int16), out: int16(-5)},
	{in: `5`, ptr: new(interface{}), out: int64(5)},
	{in: `G1.2`, ptr: new(float64), out: 1.2},
	{in: `-5`, ptr: new(int16), out: int16(-5)},
	{in: `G2`, ptr: new(interface{}), out: float64(2.0)},
	{in: `G-1.2`, ptr: new(float64), out: -1.2},
	{in: "?", ptr: new(interface{}), out: nil},
	{in: `'a\u1234'`, ptr: new(string), out: "a\u1234"},
	{in: `'http:\/\/'`, ptr: new(string), out: "http://"},
	{in: `'g-clef: \uD834\uDD1E'`, ptr: new(string), out: "g-clef: \U0001D11E"},
	{in: `'invalid: \uD834x\uDD1E'`, ptr: new(string), out: "invalid: \uFFFDx\uFFFD"},
	{in: "'badutf8: \t\r\n'", ptr: new(string), out: "badutf8: \t\r\n"},
	{in: `A<1,?,'X'= {1,2,3}, 'Y'= 4>`, ptr: new(T), out: T{Y: 4}, err: &UnmarshalTypeError{"array", reflect.TypeOf(""), 12, "T", "X"}},
	{in: `A<1,?,'X'= 23>`, ptr: new(T), out: T{}, err: &UnmarshalTypeError{"int", reflect.TypeOf(""), 13, "T", "X"}},
	{in: `A<1,?,'x'= 1>`, ptr: new(tx), out: tx{}},
	{in: `A<1,?,'x'= 1>`, ptr: new(tx), err: fmt.Errorf("oscript: unknown field \"x\""), disallowUnknownFields: true},
	{in: `A<1,?,'k1'=1,'k2'='s','k3'={1,G2.0,G3E-3},'k4'=A<1,?,'kk1'='s','kk2'=2>>`, ptr: new(interface{}), out: numAsInt64},

	// raw values with whitespace
	{in: "\n true ", ptr: new(bool), out: true},
	{in: "\t 1 ", ptr: new(int), out: 1},
	{in: "\r G1.2 ", ptr: new(float64), out: 1.2},
	{in: "\t -5 \n", ptr: new(int16), out: int16(-5)},
	{in: "\t 'a\\u1234' \n", ptr: new(string), out: "a\u1234"},

	// Z has a "-" tag.
	{in: `A<1,?,'Y'= 1, 'Z'= 2>`, ptr: new(T), out: T{Y: 1}},
	{in: `A<1,?,'Y'= 1, 'Z'= 2>`, ptr: new(T), err: fmt.Errorf("oscript: unknown field \"Z\""), disallowUnknownFields: true},

	{in: `A<1,?,'alpha'= 'abc', 'alphabet'= 'xyz'>`, ptr: new(U), out: U{Alphabet: "abc"}},
	{in: `A<1,?,'alpha'= 'abc', 'alphabet'= 'xyz'>`, ptr: new(U), err: fmt.Errorf("oscript: unknown field \"alphabet\""), disallowUnknownFields: true},
	{in: `A<1,?,'alpha'= 'abc'>`, ptr: new(U), out: U{Alphabet: "abc"}},
	{in: `A<1,?,'alphabet'= 'xyz'>`, ptr: new(U), out: U{}},
	{in: `A<1,?,'alphabet'= 'xyz'>`, ptr: new(U), err: fmt.Errorf("oscript: unknown field \"alphabet\""), disallowUnknownFields: true},

	// syntax errors
	{in: `A<1,?,'X'= 'foo', 'Y'}`, err: &SyntaxError{"invalid character '}' after object key", 22}},
	{in: `{1, 2, 3+}`, err: &SyntaxError{"invalid character '+' after array element", 9}},
	{in: `{2, 3`, err: &SyntaxError{msg: "unexpected end of Oscript input", Offset: 5}},

	// raw value errors
	{in: "\x01 42", err: &SyntaxError{"invalid character '\\x01' looking for beginning of value", 1}},
	{in: " 42 \x01", err: &SyntaxError{"invalid character '\\x01' after top-level value", 5}},
	{in: "\x01 true", err: &SyntaxError{"invalid character '\\x01' looking for beginning of value", 1}},
	{in: " false \x01", err: &SyntaxError{"invalid character '\\x01' after top-level value", 8}},
	{in: "\x01 G1.2", err: &SyntaxError{"invalid character '\\x01' looking for beginning of value", 1}},
	{in: " G3.4 \x01", err: &SyntaxError{"invalid character '\\x01' after top-level value", 7}},
	{in: "\x01 'string'", err: &SyntaxError{"invalid character '\\x01' looking for beginning of value", 1}},
	{in: " 'string' \x01", err: &SyntaxError{"invalid character '\\x01' after top-level value", 11}},

	// array tests
	{in: `{1, 2, 3}`, ptr: new([3]int), out: [3]int{1, 2, 3}},
	{in: `{1, 2, 3}`, ptr: new([1]int), out: [1]int{1}},
	{in: `{1, 2, 3}`, ptr: new([5]int), out: [5]int{1, 2, 3, 0, 0}},

	// empty array to test
	{in: `{}`, ptr: new([]interface{}), out: []interface{}{}},
	{in: `?`, ptr: new([]interface{}), out: []interface{}(nil)},
	{in: `A<1,?,'T'={}>`, ptr: new(map[string]interface{}), out: map[string]interface{}{"T": []interface{}{}}},
	{in: `A<1,?,'T'=?>`, ptr: new(map[string]interface{}), out: map[string]interface{}{"T": interface{}(nil)}},

	// composite tests
	{in: allValueIndent, ptr: new(All), out: allValue},
	{in: allValueCompact, ptr: new(All), out: allValue},
	{in: allValueIndent, ptr: new(*All), out: &allValue},
	{in: allValueCompact, ptr: new(*All), out: &allValue},
	{in: pallValueIndent, ptr: new(All), out: pallValue},
	{in: pallValueCompact, ptr: new(All), out: pallValue},
	{in: pallValueIndent, ptr: new(*All), out: &pallValue},
	{in: pallValueCompact, ptr: new(*All), out: &pallValue},

	// unmarshal interface test
	{in: `A<1,?,'T'=false>`, ptr: &um0, out: umtrue}, // use "false" so test will fail if custom unmarshaler is not called
	{in: `A<1,?,'T'=false>`, ptr: &ump, out: &umtrue},
	{in: `{A<1,?,'T'=false>}`, ptr: &umslice, out: umslice},
	{in: `{A<1,?,'T'=false>}`, ptr: &umslicep, out: &umslice},
	{in: `A<1,?,'M'=A<1,?,'T'='x:y'>>`, ptr: &umstruct, out: umstruct},

	// integer-keyed map test
	{
		in:  `A<1,?,'-1'='a','0'='b','1'='c'>`,
		ptr: new(map[int]string),
		out: map[int]string{-1: "a", 0: "b", 1: "c"},
	},
	{
		in:  `A<1,?,'0'='a','10'='c','9'='b'>`,
		ptr: new(map[u8]string),
		out: map[u8]string{0: "a", 9: "b", 10: "c"},
	},
	{
		in:  `A<1,?,'-9223372036854775808'='min','9223372036854775807'='max'>`,
		ptr: new(map[int64]string),
		out: map[int64]string{math.MinInt64: "min", math.MaxInt64: "max"},
	},
	{
		in:  `A<1,?,'18446744073709551615'='max'>`,
		ptr: new(map[uint64]string),
		out: map[uint64]string{math.MaxUint64: "max"},
	},
	{
		in:  `A<1,?,'0'=false,'10'=true>`,
		ptr: new(map[uintptr]bool),
		out: map[uintptr]bool{0: false, 10: true},
	},

	// Check that MarshalText and UnmarshalText take precedence
	// over default integer handling in map keys.
	{
		in:  `A<1,?,'u2'=4>`,
		ptr: new(map[u8marshal]int),
		out: map[u8marshal]int{2: 4},
	},
	{
		in:  `A<1,?,'2'=4>`,
		ptr: new(map[u8marshal]int),
		err: errMissingU8Prefix,
	},

	// integer-keyed map errors
	{
		in:  `A<1,?,'abc'='abc'>`,
		ptr: new(map[int]string),
		err: &UnmarshalTypeError{Value: "number abc", Type: reflect.TypeOf(0), Offset: 7},
	},
	{
		in:  `A<1,?,'256'='abc'>`,
		ptr: new(map[uint8]string),
		err: &UnmarshalTypeError{Value: "number 256", Type: reflect.TypeOf(uint8(0)), Offset: 7},
	},
	{
		in:  `A<1,?,'128'='abc'>`,
		ptr: new(map[int8]string),
		err: &UnmarshalTypeError{Value: "number 128", Type: reflect.TypeOf(int8(0)), Offset: 7},
	},
	{
		in:  `A<1,?,'-1'='abc'>`,
		ptr: new(map[uint8]string),
		err: &UnmarshalTypeError{Value: "number -1", Type: reflect.TypeOf(uint8(0)), Offset: 7},
	},

	// Map keys can be encoding.TextUnmarshalers.
	{in: `A<1,?,'x:y'=true>`, ptr: &ummapType, out: ummapXY},
	// If multiple values for the same key exists, only the most recent value is used.
	{in: `A<1,?,'x:y'=false,'x:y'=true>`, ptr: &ummapType, out: ummapXY},

	// Overwriting of data.
	// This is different from package xml, but it's what we've always done.
	// Now documented and tested.
	{in: `{2}`, ptr: sliceAddr([]int{1}), out: []int{2}},
	{in: `A<1,?,'key'= 2>`, ptr: mapAddr(map[string]int{"old": 0, "key": 1}), out: map[string]int{"key": 2}},
	{
		in: `A<1,?,
	  		'Level0'= 1,
	  		'Level1b'= 2,
	  		'Level1c'= 3,
	  		'x'= 4,
	  		'LEVEL1A'= 5,
	  		'LEVEL1B'= 6,
	  		'e'= A<1,?,
	  			'Level1a'= 8,
	  			'Level1b'= 9,
	  			'Level1c'= 10,
	  			'Level1d'= 11,
	  			'x'= 12
	  		>,
	  		'Loop1'= 13,
	  		'Loop2'= 14,
	  		'X'= 15,
	  		'Y'= 16,
	  		'Z'= 17,
	  		'Q'= 18
	  	>`,
		ptr: new(Top),
		out: Top{
			Level0: 1,
			Embed0: Embed0{
				Level1b: 2,
				Level1c: 3,
			},
			Embed0a: &Embed0a{
				Level1a: 5,
				Level1b: 6,
			},
			Embed0b: &Embed0b{
				Level1a: 8,
				Level1b: 9,
				Level1c: 10,
				Level1d: 11,
				Level1e: 12,
			},
			Loop: Loop{
				Loop1: 13,
				Loop2: 14,
			},

			Embed0p: Embed0p{
				Point: image.Point{X: 15, Y: 16},
			},

			Embed0q: Embed0q{
				Point: Point{Z: 17},
			},
			embed: embed{
				Q: 18,
			},
		},
	},
	{
		in:  `A<1,?,'hello'= 1>`,
		ptr: new(Ambig),
		out: Ambig{First: 1},
	},
	{
		in:  `A<1,?,'X'= 1,'Y'=2>`,
		ptr: new(S5),
		out: S5{S8: S8{S9: S9{Y: 2}}},
	},
	{
		in:                    `A<1,?,'X'= 1,'Y'=2>`,
		ptr:                   new(S5),
		err:                   fmt.Errorf("oscript: unknown field \"X\""),
		disallowUnknownFields: true,
	},
	{
		in:  `A<1,?, 'X'= 1,'Y'=2>`,
		ptr: new(S10),
		out: S10{S13: S13{S8: S8{S9: S9{Y: 2}}}},
	},
	{
		in:                    `A<1,?,'X'= 1,'Y'=2>`,
		ptr:                   new(S10),
		err:                   fmt.Errorf("oscript: unknown field \"X\""),
		disallowUnknownFields: true,
	},

	// invalid UTF-8 is coerced to valid UTF-8.
	{
		in:  "'hello\xffworld'",
		ptr: new(string),
		out: "hello\ufffdworld",
	},
	{
		in:  "'hello\xc2\xc2world'",
		ptr: new(string),
		out: "hello\ufffd\ufffdworld",
	},
	{
		in:  "'hello\xc2\xffworld'",
		ptr: new(string),
		out: "hello\ufffd\ufffdworld",
	},
	{
		in:  "'hello\\ud800world'",
		ptr: new(string),
		out: "hello\ufffdworld",
	},
	{
		in:  "'hello\\ud800\\ud800world'",
		ptr: new(string),
		out: "hello\ufffd\ufffdworld",
	},
	{
		in:  "'hello\\ud800\\ud800world'",
		ptr: new(string),
		out: "hello\ufffd\ufffdworld",
	},
	{
		in:  "'hello\xed\xa0\x80\xed\xb0\x80world'",
		ptr: new(string),
		out: "hello\ufffd\ufffd\ufffd\ufffd\ufffd\ufffdworld",
	},

	// oscript.Time implements encoding.TextUnmarshaler.
	/*{
		in:  `A<1,?,'Thu Mar 08 22:36:02 2018'= 'hello world'>`,
		ptr: &map[time.Time]string{},
		want: map[time.Time]string{time.Date(2018, 3, 8, 22, 36, 2, 0, time.UTC): "hello world"},
	},*/
	{
		in:  `A<1,?,'Thu Mar 08 22:36:02 2018'= 'hello world'>`,
		ptr: &map[Point]string{},
		err: &UnmarshalTypeError{Value: "object", Type: reflect.TypeOf(map[Point]string{}), Offset: 6},
	},
	{
		in:  `A<1,?,'asdf'= 'hello world'>`,
		ptr: &map[unmarshaler]string{},
		err: &UnmarshalTypeError{Value: "object", Type: reflect.TypeOf(map[unmarshaler]string{}), Offset: 6},
	},

	// time.Time
	{
		in:  `A<1,?,'date'= D/2018/3/8:22:36:2>`,
		ptr: &map[string]time.Time{},
		out: map[string]time.Time{"date": time.Date(2018, 3, 8, 22, 36, 2, 0, time.UTC)},
	},
	{
		in:  `D/1/1/1:0:0:0`,
		ptr: new(time.Time),
		out: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
	},
	// time.Time interface test
	{
		in:  `D/2018/3/8:22:36:2`,
		ptr: new(interface{}),
		out: time.Date(2018, 3, 8, 22, 36, 2, 0, time.UTC),
	},
	{
		in:  `D/1/1/1:0:0:0`,
		ptr: new(interface{}),
		out: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
	},
	{
		in:  `D/2018/3/8:22:36:2`,
		ptr: new(int),
		out: time.Date(2018, 3, 8, 22, 36, 2, 0, time.UTC),
		err: &UnmarshalTypeError{Value: "time", Type: reflect.TypeOf(int(0)), Offset: 18},
	},
	{
		in:  `A<1,?,'date'= D/2018/3/9:22:36:2>`,
		ptr: new(interface{}),
		out: map[string]interface{}{"date": time.Date(2018, 3, 9, 22, 36, 2, 0, time.UTC)},
	},
	{
		in:  `A<1,?,'date'= D/1/1/1:0:0:0>`,
		ptr: new(interface{}),
		out: map[string]interface{}{"date": time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)},
	},
	// oscript.Error
	{
		in:  `E1024`,
		ptr: new(Error),
		out: Error(1024),
	},
	// bytes
	{
		in:  `'AQID'`,
		ptr: new([]byteWithMarshalOscript),
		out: []byteWithMarshalOscript{1, 2, 3},
	},
	{
		in:     `{'Z01','Z02','Z03'}`,
		ptr:    new([]byteWithMarshalOscript),
		out:    []byteWithMarshalOscript{1, 2, 3},
		golden: true,
	},
	{
		in:  `'AQID'`,
		ptr: new([]byteWithPtrMarshalOscript),
		out: []byteWithPtrMarshalOscript{1, 2, 3},
	},
	{
		in:     `{'Z01','Z02','Z03'}`,
		ptr:    new([]byteWithPtrMarshalOscript),
		out:    []byteWithPtrMarshalOscript{1, 2, 3},
		golden: true,
	},

	// ints work with the marshaler but not the base64 []byte case
	{
		in:     `{'Z01','Z02','Z03'}`,
		ptr:    new([]intWithMarshalOscript),
		out:    []intWithMarshalOscript{1, 2, 3},
		golden: true,
	},
	{
		in:     `{'Z01','Z02','Z03'}`,
		ptr:    new([]intWithPtrMarshalOscript),
		out:    []intWithPtrMarshalOscript{1, 2, 3},
		golden: true,
	},

	{in: `G0.000001`, ptr: new(float64), out: 0.000001, golden: true},
	{in: `G1e-7`, ptr: new(float64), out: 1e-7, golden: true},
	{in: `G100000000000000000000`, ptr: new(float64), out: 100000000000000000000.0, golden: true},
	{in: `G1e+21`, ptr: new(float64), out: 1e21, golden: true},
	{in: `G-0.000001`, ptr: new(float64), out: -0.000001, golden: true},
	{in: `G-1e-7`, ptr: new(float64), out: -1e-7, golden: true},
	{in: `G-100000000000000000000`, ptr: new(float64), out: -100000000000000000000.0, golden: true},
	{in: `G-1e+21`, ptr: new(float64), out: -1e21, golden: true},
	{in: `G999999999999999900000`, ptr: new(float64), out: 999999999999999900000.0, golden: true},
	{in: `G9007199254740992`, ptr: new(float64), out: 9007199254740992.0, golden: true},
	{in: `G9007199254740993`, ptr: new(float64), out: 9007199254740992.0, golden: false},

	{
		in:  `A<1,?,'V'= A<1,?,'F2'= 'hello'>>`,
		ptr: new(VOuter),
		err: &UnmarshalTypeError{
			Value:  "string",
			Struct: "V",
			Field:  "F2",
			Type:   reflect.TypeOf(int32(0)),
			Offset: 30,
		},
	},
	{
		in:  `A<1,?,'V'= A<1,?,'F4'= A<1,?>, 'F2'= 'hello'>>`,
		ptr: new(VOuter),
		err: &UnmarshalTypeError{
			Value:  "string",
			Struct: "V",
			Field:  "F2",
			Type:   reflect.TypeOf(int32(0)),
			Offset: 44,
		},
	},

	// additional tests for disallowUnknownFields
	{
		in: `A<1,?,
			'Level0'= 1,
			'Level1b'= 2,
			'Level1c'= 3,
			'x'= 4,
			'Level1a'= 5,
			'LEVEL1B'= 6,
			'e'= A<1,?,
				'Level1a'= 8,
				'Level1b'= 9,
				'Level1c'= 10,
				'Level1d'= 11,
				'x'= 12
			>,
			'Loop1'= 13,
			'Loop2'= 14,
			'X'= 15,
			'Y'= 16,
			'Z'= 17,
			'Q'= 18,
			'extra'= true
		>`,
		ptr:                   new(Top),
		err:                   fmt.Errorf("oscript: unknown field \"extra\""),
		disallowUnknownFields: true,
	},
	{
		in: `A<1,?,
			'Level0'= 1,
			'Level1b'= 2,
			'Level1c'= 3,
			'x'= 4,
			'Level1a'= 5,
			'LEVEL1B'= 6,
			'e'= A<1,?,
				'Level1a'= 8,
				'Level1b'= 9,
				'Level1c'= 10,
				'Level1d'= 11,
				'x'= 12,
				'extra'= ?
			>,
			'Loop1'= 13,
			'Loop2'= 14,
			'X'= 15,
			'Y'= 16,
			'Z'= 17,
			'Q'= 18
		>`,
		ptr:                   new(Top),
		err:                   fmt.Errorf("oscript: unknown field \"extra\""),
		disallowUnknownFields: true,
	},

	// oscript.SDOName
	{
		in:                    `A<1,?,'Age'=6,'_SDOName'='world.gopher'>`,
		ptr:                   new(sdoNameTag),
		out:                   sdoNameTag{Age: 6},
		disallowUnknownFields: true,
	},
	{
		in:                    `A<1,?,'Age'=6,'_SDOName'='world.gopher','Home'='LA'>`,
		ptr:                   new(sdoNameTag),
		err:                   fmt.Errorf("oscript: unknown field \"Home\""),
		disallowUnknownFields: true,
	},

	{ // error
		in:  `A<1,?,'Age'=6,'_SDOName'='world.go'>`,
		ptr: new(sdoNameTag),
		err: fmt.Errorf("oscript: unknown SDOName \"world.go\""),
	},
	/*{ // expects SDOName
		in:  `A<1,?,'Age'=6>`,
		ptr: new(gopher),
		err: fmt.Errorf("oscript: no compares data object with type Objecter"),
	}, */

	// UnmarshalTypeError without field & struct values
	{
		in:  `A<1,?,'data'=A<1,?,'test1'= 'bob', 'test2'= 123>>`,
		ptr: new(mapStringToStringData),
		err: &UnmarshalTypeError{Value: "int", Type: reflect.TypeOf(""), Offset: 47, Struct: "mapStringToStringData", Field: "data"},
	},
	{
		in:  `A<1,?,'data'=A<1,?,'test1'= 123, 'test2'= 'bob'>>`,
		ptr: new(mapStringToStringData),
		err: &UnmarshalTypeError{Value: "int", Type: reflect.TypeOf(""), Offset: 31, Struct: "mapStringToStringData", Field: "data"},
	},
}

func TestMarshal(t *testing.T) {
	b, err := Marshal(allValue)
	require.Nil(t, err, "allValue")
	assert.Equal(t, allValueCompact, string(b), "allValueCompact")

	b, err = Marshal(pallValue)
	require.Nil(t, err, "pallValue")
	assert.Equal(t, pallValueCompact, string(b), "pallValueCompact")
}

var badUTF8 = []struct {
	in, out string
}{
	{"hello\xffworld", `'hello\ufffdworld'`},
	{"", `''`},
	{"\xff", `'\ufffd'`},
	{"\xff\xff", `'\ufffd\ufffd'`},
	{"a\xffb", `'a\ufffdb'`},
	{"\xe6\x97\xa5\xe6\x9c\xac\xff\xaa\x9e", `'日本\ufffd\ufffd\ufffd'`},
	{"\t hello \n", `'\t hello \n'`},
}

func TestMarshalBadUTF8(t *testing.T) {
	for _, tt := range badUTF8 {
		b, err := Marshal(tt.in)
		require.Nil(t, err)
		assert.Equal(t, tt.out, string(b))
	}
}

func TestMarshalEmbeds(t *testing.T) {
	top := &Top{
		Level0: 1,
		Embed0: Embed0{
			Level1b: 2,
			Level1c: 3,
		},

		Embed0a: &Embed0a{
			Level1a: 5,
			Level1b: 6,
		},

		Embed0b: &Embed0b{
			Level1a: 8,
			Level1b: 9,
			Level1c: 10,
			Level1d: 11,
			Level1e: 12,
		},

		Loop: Loop{
			Loop1: 13,
			Loop2: 14,
		},

		Embed0p: Embed0p{
			Point: image.Point{X: 15, Y: 16},
		},

		Embed0q: Embed0q{
			Point: Point{Z: 17},
		},

		embed: embed{
			Q: 18,
		},
	}
	b, err := Marshal(top)
	require.Nil(t, err)

	want := "A<1,?,'Level0'=1,'Level1b'=2,'Level1c'=3,'Level1a'=5,'LEVEL1B'=6,'e'=A<1,?,'Level1a'=8,'Level1b'=9,'Level1c'=10,'Level1d'=11,'x'=12>,'Loop1'=13,'Loop2'=14,'X'=15,'Y'=16,'Z'=17,'Q'=18>"
	assert.Equal(t, want, string(b))
}

func TestUnmarshal(t *testing.T) {
	for i, tt := range unmarshalTests {
		var scan scanner
		in := []byte(tt.in)
		if err := checkValid(in, &scan); err != nil {
			if !assert.Equal(t, tt.err, err, "checkValid") {
				continue
			}
		}
		if tt.ptr == nil {
			continue
		}

		v := reflect.New(reflect.TypeOf(tt.ptr).Elem())
		if tt.indirect {
			indirect := reflect.Indirect(reflect.ValueOf(tt.ptr))
			v = reflect.New(indirect.Type())
			v.Elem().Set(reflect.ValueOf(indirect.Interface()))
		}

		dec := NewDecoder(bytes.NewReader(in))
		if tt.disallowUnknownFields {
			dec.DisallowUnknownFields()
		}

		if err := dec.Decode(v.Interface()); !assert.Equal(t, tt.err, err) {
			continue
		} else if err != nil {
			continue
		}

		if !assert.Equal(t, tt.out, v.Elem().Interface(), "#%d: mismatch", i) {
			data, _ := Marshal(v.Elem().Interface())
			println(string(data))
			data, _ = Marshal(tt.out)
			println(string(data))
			continue
		}

		// Error round trip also decodes correctly.
		if tt.err == nil {
			enc, err := Marshal(v.Interface())
			if !assert.Nil(t, err, "#%d: re-marshaling", i) {
				continue
			}

			if tt.golden && !bytes.Equal(enc, in) {
				t.Errorf("#%d: remarshal mismatch:\nhave: %s\nwant: %s", i, enc, in)
			}

			vv := reflect.New(reflect.TypeOf(tt.ptr).Elem())
			if tt.indirect {
				indirect := reflect.Indirect(reflect.ValueOf(tt.ptr))
				vv = reflect.New(indirect.Type())
				vv.Elem().Set(reflect.ValueOf(indirect.Interface()))
			}

			dec = NewDecoder(bytes.NewReader(enc))
			if err := dec.Decode(vv.Interface()); err != nil {
				t.Errorf("#%d: error re-unmarshaling %#q: %v", i, enc, err)
				continue
			}

			if !assert.Equal(t, vv.Elem().Interface(), v.Elem().Interface(), "#%d: mismatch", i) {
				t.Errorf("     In: %q", strings.Map(noSpace, string(in)))
				t.Errorf("Marshal: %q", strings.Map(noSpace, string(enc)))
				continue
			}
		}
	}
}

func TestUnmarshalMarshal(t *testing.T) {
	initBig()
	var v interface{}
	if err := Unmarshal(oscriptBig, &v); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	b, err := Marshal(v)
	require.Nil(t, err)
	assert.Equal(t, oscriptBig, b)
}

func TestLargeByteSlice(t *testing.T) {
	s0 := make([]byte, 2000)
	for i := range s0 {
		s0[i] = byte(i)
	}
	b, err := Marshal(s0)
	require.Nil(t, err)

	var s1 []byte
	err = Unmarshal(b, &s1)
	require.Nil(t, err)
	assert.Equal(t, s0, s1)
}

type Xint struct {
	X int
}

func TestUnmarshalInterface(t *testing.T) {
	var xint Xint
	var i interface{} = &xint
	err := Unmarshal([]byte(`A<1,?,'X'=1>`), &i)

	require.Nil(t, err)
	assert.Equal(t, 1, xint.X, "did not write to xint")
}

func TestUnmarshalPtrPtr(t *testing.T) {
	var xint Xint
	pxint := &xint
	err := Unmarshal([]byte(`A<1,?,'X'=1>`), &pxint)

	require.Nil(t, err)
	assert.Equal(t, 1, xint.X, "did not write to xint")
}

func noSpace(c rune) rune {
	if isSpace(byte(c)) { //only used for ascii
		return -1
	}
	return c
}

type All struct {
	Bool    bool
	Int     int
	Int8    int8
	Int16   int16
	Int32   int32
	Int64   int64
	Uint    uint
	Uint8   uint8
	Uint16  uint16
	Uint32  uint32
	Uint64  uint64
	Uintptr uintptr
	Float32 float32
	Float64 float64

	Foo  string `oscript:"bar"`
	Foo2 string `oscript:"bar2,dummyopt"`

	PBool    *bool
	PInt     *int
	PInt8    *int8
	PInt16   *int16
	PInt32   *int32
	PInt64   *int64
	PUint    *uint
	PUint8   *uint8
	PUint16  *uint16
	PUint32  *uint32
	PUint64  *uint64
	PUintptr *uintptr
	PFloat32 *float32
	PFloat64 *float64

	String  string
	PString *string

	Map   map[string]Small
	MapP  map[string]*Small
	PMap  *map[string]Small
	PMapP *map[string]*Small

	EmptyMap map[string]Small
	NilMap   map[string]Small

	Slice   []Small
	SliceP  []*Small
	PSlice  *[]Small
	PSliceP *[]*Small

	EmptySlice []Small
	NilSlice   []Small

	StringSlice []string
	ByteSlice   []byte

	Small   Small
	PSmall  *Small
	PPSmall **Small

	Interface  interface{}
	PInterface *interface{}

	unexported int
}

type Small struct {
	Tag string
}

var allValue = All{
	Bool:    true,
	Int:     2,
	Int8:    3,
	Int16:   4,
	Int32:   5,
	Int64:   6,
	Uint:    7,
	Uint8:   8,
	Uint16:  9,
	Uint32:  10,
	Uint64:  11,
	Uintptr: 12,
	Float32: 14.1,
	Float64: 15.1,
	Foo:     "foo",
	Foo2:    "foo2",
	String:  "16",
	Map: map[string]Small{
		"17": {Tag: "tag17"},
		"18": {Tag: "tag18"},
	},
	MapP: map[string]*Small{
		"19": {Tag: "tag19"},
		"20": nil,
	},
	EmptyMap:    map[string]Small{},
	Slice:       []Small{{Tag: "tag20"}, {Tag: "tag21"}},
	SliceP:      []*Small{{Tag: "tag22"}, nil, {Tag: "tag23"}},
	EmptySlice:  []Small{},
	StringSlice: []string{"str24", "str25", "str26"},
	ByteSlice:   []byte{27, 28, 29},
	Small:       Small{Tag: "tag30"},
	PSmall:      &Small{Tag: "tag31"},
	Interface:   5.2,
}

var pallValue = All{
	PBool:      &allValue.Bool,
	PInt:       &allValue.Int,
	PInt8:      &allValue.Int8,
	PInt16:     &allValue.Int16,
	PInt32:     &allValue.Int32,
	PInt64:     &allValue.Int64,
	PUint:      &allValue.Uint,
	PUint8:     &allValue.Uint8,
	PUint16:    &allValue.Uint16,
	PUint32:    &allValue.Uint32,
	PUint64:    &allValue.Uint64,
	PUintptr:   &allValue.Uintptr,
	PFloat32:   &allValue.Float32,
	PFloat64:   &allValue.Float64,
	PString:    &allValue.String,
	PMap:       &allValue.Map,
	PMapP:      &allValue.MapP,
	PSlice:     &allValue.Slice,
	PSliceP:    &allValue.SliceP,
	PPSmall:    &allValue.PSmall,
	PInterface: &allValue.Interface,
}

var allValueIndent = `A<1,?,
	'Bool'= true,
	'Int'= 2,
	'Int8'= 3,
	'Int16'= 4,
	'Int32'= 5,
	'Int64'= 6,
	'Uint'= 7,
	'Uint8'= 8,
	'Uint16'= 9,
	'Uint32'= 10,
	'Uint64'= 11,
	'Uintptr'= 12,
	'Float32'= G14.1,
	'Float64'= G15.1,
	'bar'= 'foo',
	'bar2'= 'foo2',
	'PBool'= ?,
	'PInt'= ?,
	'PInt8'= ?,
	'PInt16'= ?,
	'PInt32'= ?,
	'PInt64'= ?,
	'PUint'= ?,
	'PUint8'= ?,
	'PUint16'= ?,
	'PUint32'= ?,
	'PUint64'= ?,
	'PUintptr'= ?,
	'PFloat32'= ?,
	'PFloat64'= ?,
	'String'= '16',
	'PString'= ?,
	'Map'= A<1,?,
		'17'= A<1,?,
			'Tag'= 'tag17'
		>,
		'18'= A<1,?,
			'Tag'= 'tag18'
		>
	>,
	'MapP'= A<1,?,
		'19'= A<1,?,
			'Tag'= 'tag19'
		>,
		'20'= ?
	>,
	'PMap'= ?,
	'PMapP'= ?,
	'EmptyMap'= A<1,?>,
	'NilMap'= ?,
	'Slice'= {
		A<1,?,
			'Tag'= 'tag20'
		>,
		A<1,?,
			'Tag'= 'tag21'
		>
	},
	'SliceP'= {
		A<1,?,
			'Tag'= 'tag22'
		>,
		?,
		A<1,?,
			'Tag'= 'tag23'
		>
	},
	'PSlice'= ?,
	'PSliceP'= ?,
	'EmptySlice'= {},
	'NilSlice'= ?,
	'StringSlice'= {
		'str24',
		'str25',
		'str26'
	},
	'ByteSlice'= 'Gxwd',
	'Small'= A<1,?,
		'Tag'= 'tag30'
	>,
	'PSmall'= A<1,?,
		'Tag'= 'tag31'
	>,
	'PPSmall'= ?,
	'Interface'= G5.2,
	'PInterface'= ?
>`

var allValueCompact = strings.Map(noSpace, allValueIndent)

var pallValueIndent = `A<1,?,
	'Bool'= false,
	'Int'= 0,
	'Int8'= 0,
	'Int16'= 0,
	'Int32'= 0,
	'Int64'= 0,
	'Uint'= 0,
	'Uint8'= 0,
	'Uint16'= 0,
	'Uint32'= 0,
	'Uint64'= 0,
	'Uintptr'= 0,
	'Float32'= G0,
	'Float64'= G0,
	'bar'= '',
	'bar2'= '',
	'PBool'= true,
	'PInt'= 2,
	'PInt8'= 3,
	'PInt16'= 4,
	'PInt32'= 5,
	'PInt64'= 6,
	'PUint'= 7,
	'PUint8'= 8,
	'PUint16'= 9,
	'PUint32'= 10,
	'PUint64'= 11,
	'PUintptr'= 12,
	'PFloat32'= G14.1,
	'PFloat64'= G15.1,
	'String'= '',
	'PString'= '16',
	'Map'= ?,
	'MapP'= ?,
	'PMap'= A<1,?,
		'17'= A<1,?,
			'Tag'= 'tag17'
		>,
		'18'= A<1,?,
			'Tag'= 'tag18'
		>
	>,
	'PMapP'= A<1,?,
		'19'= A<1,?,
			'Tag'= 'tag19'
		>,
		'20'= ?
	>,
	'EmptyMap'= ?,
	'NilMap'= ?,
	'Slice'= ?,
	'SliceP'= ?,
	'PSlice'= {
		A<1,?,
			'Tag'= 'tag20'
		>,
		A<1,?,
			'Tag'= 'tag21'
		>
	},
	'PSliceP'= {
		A<1,?,
			'Tag'= 'tag22'
		>,
		?,
		A<1,?,
			'Tag'= 'tag23'
		>
	},
	'EmptySlice'= ?,
	'NilSlice'= ?,
	'StringSlice'= ?,
	'ByteSlice'= ?,
	'Small'= A<1,?,
		'Tag'= ''
	>,
	'PSmall'= ?,
	'PPSmall'= A<1,?,
		'Tag'= 'tag31'
	>,
	'Interface'= ?,
	'PInterface'= G5.2
>`

var pallValueCompact = strings.Map(noSpace, pallValueIndent)

func TestRefUnmarshal(t *testing.T) {
	type S struct {
		// Ref is defined in encode_test.go.
		R0 Ref
		R1 *Ref
	}

	want := S{
		R0: 12,
		R1: new(Ref),
	}

	*want.R1 = 12
	var got S

	if err := Unmarshal([]byte(`A<1,?,'R0'='ref','R1'='ref'>`), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func intp(x int) *int {
	p := new(int)
	*p = x
	return p
}

func intpp(x *int) **int {
	pp := new(*int)
	*pp = x
	return pp
}

var interfaceSetTests = []struct {
	pre     interface{}
	oscript string
	post    interface{}
}{
	{"foo", `'bar'`, "bar"},
	{"foo", `2`, int64(2)},
	{"foo", `true`, true},
	{"foo", `?`, nil},
	{nil, `?`, nil},
	{new(int), `?`, nil},
	{(*int)(nil), `?`, nil},
	{new(*int), `?`, new(*int)},
	{(**int)(nil), `?`, nil},
	{intp(1), `?`, nil},
	{intpp(nil), `?`, intpp(nil)},
	{intpp(intp(1)), `?`, intpp(nil)},
}

func TestInterfaceSet(t *testing.T) {
	for _, tt := range interfaceSetTests {
		b := struct{ X interface{} }{tt.pre}
		blob := `A<1,?,'X'=` + tt.oscript + `>`
		if err := Unmarshal([]byte(blob), &b); err != nil {
			t.Errorf("Unmarshal %#q: %v", blob, err)
			continue
		}

		if !reflect.DeepEqual(b.X, tt.post) {
			t.Errorf("Unmarshal %#q into %#v: X=%#v, want %#v", blob, tt.pre, b.X, tt.post)
		}
	}
}

type UndefinedTest struct {
	Bool      bool
	Int       int
	Int8      int8
	Int16     int16
	Int32     int32
	Int64     int64
	Uint      uint
	Uint8     uint8
	Uint16    uint16
	Uint32    uint32
	Uint64    uint64
	Float32   float32
	Float64   float64
	String    string
	PBool     *bool
	Map       map[string]string
	Slice     []string
	Interface interface{}
	PTime     *time.Time
	PBigInt   *big.Int
	PText     *MustNotUnmarshalText
	PBuffer   *bytes.Buffer // has methods, just not relevant ones
	PStruct   *struct{}
	Time      time.Time
	BigInt    big.Int
	Buffer    bytes.Buffer
	Struct    struct{}
}

// Oscript undfeind values should be ignored for primitives and string values instead of resulting in an error.
func TestUnmarshalUndefined(t *testing.T) {
	// Unmarshal:
	// The Oscript undefined value unmarshals into an interface, map, pointer, or slice
	// by setting that Go value to nil. Because undefined is often used in Oscript to mean
	// ``not present,'' unmarshaling a Oscript undefined into any other Go type has no effect
	// on the value and produces no error.
	oscriptData := []byte(`A<1,?,
				'Bool'    = ?,
				'Int'     = ?,
				'Int8'    = ?,
				'Int16'   = ?,
				'Int32'   = ?,
				'Int64'   = ?,
				'Uint'    = ?,
				'Uint8'   = ?,
				'Uint16'  = ?,
				'Uint32'  = ?,
				'Uint64'  = ?,
				'Float32' = ?,
				'Float64' = ?,
				'String'  = ?,
				'PBool'= ?,
				'Map'= ?,
				'Slice'= ?,
				'Interface'= ?,
				'PTime'= ?,
				'PBigInt'= ?,
				'PText'= ?,
				'PBuffer'= ?,
				'PStruct'= ?,
				'Time'= ?,
				'BigInt'= ?,
				'Text'= ?,
				'Buffer'= ?,
				'Struct'= ?
			>`)

	undefineds := UndefinedTest{
		Bool:      true,
		Int:       2,
		Int8:      3,
		Int16:     4,
		Int32:     5,
		Int64:     6,
		Uint:      7,
		Uint8:     8,
		Uint16:    9,
		Uint32:    10,
		Uint64:    11,
		Float32:   12.1,
		Float64:   13.1,
		String:    "14",
		PBool:     new(bool),
		Map:       map[string]string{},
		Slice:     []string{},
		Interface: new(MustNotUnmarshalOscript),
		PTime:     new(time.Time),
		PBigInt:   new(big.Int),
		PText:     new(MustNotUnmarshalText),
		PStruct:   new(struct{}),
		PBuffer:   new(bytes.Buffer),
		Time:      time.Unix(123456789, 0),
		BigInt:    *big.NewInt(123),
	}

	before := undefineds.Time.String()
	err := Unmarshal(oscriptData, &undefineds)

	if err != nil {
		t.Errorf("Unmarshal of undefined values failed: %v", err)
	}

	if !undefineds.Bool || undefineds.Int != 2 || undefineds.Int8 != 3 || undefineds.Int16 != 4 || undefineds.Int32 != 5 || undefineds.Int64 != 6 ||
		undefineds.Uint != 7 || undefineds.Uint8 != 8 || undefineds.Uint16 != 9 || undefineds.Uint32 != 10 || undefineds.Uint64 != 11 ||
		undefineds.Float32 != 12.1 || undefineds.Float64 != 13.1 || undefineds.String != "14" {
		t.Errorf("Unmarshal of undefined values affected primitives")
	}

	if undefineds.PBool != nil {
		t.Errorf("Unmarshal of undefined did not clear undefined.PBool")
	}

	if undefineds.Map != nil {
		t.Errorf("Unmarshal of undefined did not clear undefined.Map")
	}

	if undefineds.Slice != nil {
		t.Errorf("Unmarshal of undefined did not clear undefined.Slice")
	}

	if undefineds.Interface != nil {
		t.Errorf("Unmarshal of undefined did not clear undefined.Interface")
	}

	if undefineds.PTime != nil {
		t.Errorf("Unmarshal of undefined did not clear undefined.PTime")
	}

	if undefineds.PBigInt != nil {
		t.Errorf("Unmarshal of undefined did not clear undefined.PBigInt")
	}

	if undefineds.PText != nil {
		t.Errorf("Unmarshal of undefined did not clear undefined.PText")
	}

	if undefineds.PBuffer != nil {
		t.Errorf("Unmarshal of undefined did not clear undefined.PBuffer")
	}

	if undefineds.PStruct != nil {
		t.Errorf("Unmarshal of undefined did not clear undefined.PStruct")
	}

	if undefineds.Time.String() != before {
		t.Errorf("Unmarshal of time.Time undefined set time to %v", undefineds.Time.String())
	}

	if undefineds.BigInt.String() != "123" {
		t.Errorf("Unmarshal of big.Int undefined set int to %v", undefineds.BigInt.String())
	}
}

type MustNotUnmarshalOscript struct{}

func (x MustNotUnmarshalOscript) UnmarshalOscript(data []byte) error {
	return errors.New("MustNotUnmarshalOscript was used")
}

type MustNotUnmarshalText struct{}

func (x MustNotUnmarshalText) UnmarshalText(text []byte) error {
	return errors.New("MustNotUnmarshalText was used")
}

func TestStringKind(t *testing.T) {
	type stringKind string
	var m1, m2 map[stringKind]int

	m1 = map[stringKind]int{
		"foo": 42,
	}
	data, err := Marshal(m1)

	if err != nil {
		t.Errorf("Unexpected error marshaling: %v", err)
	}

	err = Unmarshal(data, &m2)
	if err != nil {
		t.Errorf("Unexpected error unmarshaling: %v", err)
	}

	if !reflect.DeepEqual(m1, m2) {
		t.Error("Items should be equal after encoding and then decoding")
	}
}

func TestByteKind(t *testing.T) {
	type byteKind []byte
	a := byteKind("hello")
	data, err := Marshal(a)
	if err != nil {
		t.Error(err)
	}
	var b byteKind
	err = Unmarshal(data, &b)

	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("expected %v == %v", a, b)
	}
}

func TestSliceOfCustomByte(t *testing.T) {
	type Uint8 uint8
	a := []Uint8("hello")

	data, err := Marshal(a)

	if err != nil {
		t.Fatal(err)
	}

	var b []Uint8
	err = Unmarshal(data, &b)

	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(a, b) {
		t.Fatalf("expected %v == %v", a, b)
	}
}

var decodeTypeErrorTests = []struct {
	dest interface{}
	src  string
}{
	{new(string), `A<1,?,'user'= 'name'>`},
	{new(error), `A<1,?>`},
	{new(error), `{}`},
	{new(error), `''`},
	{new(error), `123`},
	{new(error), `true`},
}

func TestUnmarshalTypeError(t *testing.T) {
	for _, item := range decodeTypeErrorTests {
		err := Unmarshal([]byte(item.src), item.dest)
		if _, ok := err.(*UnmarshalTypeError); !ok {
			t.Errorf("expected type error for Unmarshal(%q, type %T): got %T",
				item.src, item.dest, err)
		}
	}
}

var unmarshalSyntaxTests = []string{
	"tru",
	"fals",
	"123e",
	`'hello`,
	`{1,2,3`,
	`A<1,?,'key'=1`,
	`A<1,'key'=1,`,
}

func TestUnmarshalSyntax(t *testing.T) {
	var x interface{}
	for _, src := range unmarshalSyntaxTests {
		err := Unmarshal([]byte(src), &x)
		if _, ok := err.(*SyntaxError); !ok {
			t.Errorf("expected syntax error for Unmarshal(%q): got %T", src, err)
		}
	}
}

// Test handling of unexported fields that should be ignored.
type unexportedFields struct {
	Name string
	m    map[string]interface{} `oscript:"-"`
	m2   map[string]interface{} `oscript:"abcd"`
}

func TestUnmarshalUnexported(t *testing.T) {
	input := `A<1,?,'Name'= 'Bob', 'm'= A<1,?,'x'= 123>, 'm2'= A<1,?,'y'= 456>, 'abcd'= A<1,?,'z'= 789>>`
	want := &unexportedFields{Name: "Bob"}

	out := &unexportedFields{}
	err := Unmarshal([]byte(input), out)

	if err != nil {
		t.Errorf("got error %v, expected nil", err)
	}

	if !reflect.DeepEqual(out, want) {
		t.Errorf("got %q, want %q", out, want)
	}
}

// Test that extra object elements in an array do not result in a
// "data changing underfoot" error.
func TestSkipArrayObjects(t *testing.T) {
	oscript := `{A<1,?>}`
	var dest [0]interface{}

	err := Unmarshal([]byte(oscript), &dest)

	if err != nil {
		t.Errorf("got error %q, want nil", err)
	}
}

type XYZ struct {
	X interface{}
	Y interface{}
	Z interface{}
	A interface{}
}

// Test semantics of pre-filled struct fields and pre-filled map fields.
func TestPrefilled(t *testing.T) {
	ptrToMap := func(m map[string]interface{}) *map[string]interface{} { return &m }

	// Values here change, cannot reuse table across runs.
	var prefillTests = []struct {
		in  string
		ptr interface{}
		out interface{}
	}{
		{
			in:  `A<1,?,'X'= 1, 'Y'= 2, 'Z'=G1.5>`,
			ptr: &XYZ{X: float32(3), Y: int16(4), Z: 1.5, A: 3.2},
			out: &XYZ{X: int64(1), Y: int64(2), Z: float64(1.5), A: 3.2},
		},
		{
			in:  `A<1,?,'X'= 1, 'Y'= 2, 'Z'=G1.5>`,
			ptr: ptrToMap(map[string]interface{}{"X": float32(3), "Y": int16(4), "Z": 1.5, "A": 3.2}),
			out: ptrToMap(map[string]interface{}{"X": int64(1), "Y": int64(2), "Z": float64(1.5), "A": 3.2}),
		},
	}

	for _, tt := range prefillTests {
		ptrstr := fmt.Sprintf("%v", tt.ptr)
		err := Unmarshal([]byte(tt.in), tt.ptr) // tt.ptr edited here
		if err != nil {
			t.Errorf("Unmarshal: %v", err)
		}

		if !reflect.DeepEqual(tt.ptr, tt.out) {
			t.Errorf("Unmarshal(%#q, %s): have %v, want %v", tt.in, ptrstr, tt.ptr, tt.out)
		}
	}
}

var invalidUnmarshalTests = []struct {
	v    interface{}
	want string
}{
	{nil, "oscript: Unmarshal(nil)"},
	{struct{}{}, "oscript: Unmarshal(non-pointer struct {})"},
	{(*int)(nil), "oscript: Unmarshal(nil *int)"},
}

func TestInvalidUnmarshal(t *testing.T) {
	buf := []byte(`A<1,?,'a'='1'>`)
	for _, tt := range invalidUnmarshalTests {
		err := Unmarshal(buf, tt.v)

		if err == nil {
			t.Errorf("Unmarshal expecting error, got nil")
			continue
		}

		if got := err.Error(); got != tt.want {
			t.Errorf("Unmarshal = %q; want %q", got, tt.want)
		}
	}
}

var invalidUnmarshalTextTests = []struct {
	v    interface{}
	want string
}{

	{nil, "oscript: Unmarshal(nil)"},
	{struct{}{}, "oscript: Unmarshal(non-pointer struct {})"},
	{(*int)(nil), "oscript: Unmarshal(nil *int)"},
	{new(net.IP), "oscript: cannot unmarshal int into Go value of type net.IP"},
}

func TestInvalidUnmarshalText(t *testing.T) {
	buf := []byte(`123`)
	for _, tt := range invalidUnmarshalTextTests {
		err := Unmarshal(buf, tt.v)
		if err == nil {
			t.Errorf("Unmarshal expecting error, got nil")
			continue
		}

		if got := err.Error(); got != tt.want {
			t.Errorf("Unmarshal = %q; want %q", got, tt.want)
		}
	}
}

// Test unmarshal behavior with regards to embedded unexported structs.
//
// (Issue 21357) If the embedded struct is a pointer and is unallocated,
// this returns an error because unmarshal cannot set the field.
//
// (Issue 24152) If the embedded struct is given an explicit name,
// ensure that the normal unmarshal logic does not panic in reflect.
func TestUnmarshalEmbeddedUnexported(t *testing.T) {
	type (
		embed1 struct{ Q int }
		embed2 struct{ Q int }

		S1 struct {
			*embed1
			R int
		}

		S2 struct {
			*embed1
			Q int
		}

		S3 struct {
			embed1
			R int
		}

		S4 struct {
			*embed1
			embed2
		}

		S6 struct {
			embed1 `oscript:"embed1"`
		}

		S7 struct {
			embed1 `oscript:"embed1"`
			embed2
		}

		S8 struct {
			embed1 `oscript:"embed1"`
			embed2 `oscript:"embed2"`
			Q      int
		}
		S9 struct {
			unexportedWithMethods `oscript:"embed"`
		}
	)

	tests := []struct {
		in  string
		ptr interface{}
		out interface{}
		err error
	}{{
		// Error since we cannot set S1.embed1, but still able to set S1.R.
		in:  `A<1,?,'R'=2,'Q'=1>`,
		ptr: new(S1),
		out: &S1{R: 2},
		err: fmt.Errorf("oscript: cannot set embedded pointer to unexported struct: oscript.embed1"),
	}, {
		// The top level Q field takes precedence.
		in:  `A<1,?,'Q'=1>`,
		ptr: new(S2),
		out: &S2{Q: 1},
	}, {
		// No issue with non-pointer variant.
		in:  `A<1,?,'R'=2,'Q'=1>`,
		ptr: new(S3),
		out: &S3{embed1: embed1{Q: 1}, R: 2},
	}, {
		// No error since both embedded structs have field R, which annihilate each other.
		// Thus, no attempt is made at setting S4.embed1.
		in:  `A<1,?,'R'=2>`,
		ptr: new(S4),
		out: new(S4),
	}, {
		// Issue 24152, ensure decodeState.indirect does not panic.
		in:  `A<1,?,'embed1'=A<1,?,'Q'=1>>`,
		ptr: new(S6),
		out: &S6{embed1{1}},
	}, {
		// Issue 24153, check that we can still set forwarded fields even in
		// the presence of a name conflict.
		//
		// This relies on obscure behavior of reflect where it is possible
		// to set a forwarded exported field on an unexported embedded struct
		// even though there is a name conflict, even when it would have been
		// impossible to do so according to Go visibility rules.
		// Go forbids this because it is ambiguous whether S7.Q refers to
		// S7.embed1.Q or S7.embed2.Q. Since embed1 and embed2 are unexported,
		// it should be impossible for an external package to set either Q.
		//
		// It is probably okay for a future reflect change to break this.
		in:  `A<1,?,'embed1'=A<1,?,'Q'=1>,'Q'=2>`,
		ptr: new(S7),
		out: &S7{embed1{1}, embed2{2}},
	}, {
		// Issue 24153, similar to the S7 case.
		in:  `A<1,?,'embed1'=A<1,?,'Q'=1>,'embed2'=A<1,?,'Q'=2>,'Q'=3>`,
		ptr: new(S8),
		out: &S8{embed1{1}, embed2{2}, 3},
	},
		{
			// Issue 228145, similar to the cases above.
			in:  `A<1,?,'embed'= A<1,?>>`,
			ptr: new(S9),
			out: &S9{},
		}}

	for i, tt := range tests {
		err := Unmarshal([]byte(tt.in), tt.ptr)
		if !reflect.DeepEqual(err, tt.err) {
			t.Errorf("#%d: %v, want %v", i, err, tt.err)
		}

		if !reflect.DeepEqual(tt.ptr, tt.out) {
			t.Errorf("#%d: mismatch\ngot:  %#+v\nwant: %#+v", i, tt.ptr, tt.out)
		}
	}
}

type unmarshalPanic struct{}

func (unmarshalPanic) UnmarshalOscript([]byte) error { panic(0xdead) }

func TestUnmarshalPanic(t *testing.T) {
	defer func() {
		if got := recover(); !reflect.DeepEqual(got, 0xdead) {
			t.Errorf("panic() = (%T)(%v), want 0xdead", got, got)
		}
	}()

	Unmarshal([]byte("{}"), &unmarshalPanic{})
	t.Fatalf("Unmarshal should have panicked")
}

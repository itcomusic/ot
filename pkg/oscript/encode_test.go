package oscript

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Optionals struct {
	Sr string `oscript:"sr"`
	So string `oscript:"so,omitempty"`
	Sw string `oscript:"-"`

	Ir int `oscript:"omitempty"`
	Io int `oscript:"io,omitempty"`

	Mr map[string]interface{} `oscript:"mr"`
	Mo map[string]interface{} `oscript:",omitempty"`

	Fr float64 `oscript:"fr"`
	Fo float64 `oscript:"fo,omitempty"`

	Br bool `oscript:"br"`
	Bo bool `oscript:"bo,omitempty"`

	Ur uint `oscript:"ur"`
	Uo uint `oscript:"uo,omitempty"`

	Str struct{} `oscript:"str"`
	Sto struct{} `oscript:"sto,omitempty"`
}

func TestOmitEmpty(t *testing.T) {
	var o Optionals
	o.Sw = "something"
	o.Mr = map[string]interface{}{}
	o.Mo = map[string]interface{}{}

	b, err := Marshal(&o)
	require.Nil(t, err)
	want := `A<1,?,'sr'='','omitempty'=0,'mr'=A<1,?>,'fr'=G0,'br'=false,'ur'=0,'str'=A<1,?>,'sto'=A<1,?>>`
	assert.Equal(t, want, string(b))
}

type Public struct {
	a bool `oscript:"A,public"`
	B bool `oscript:",public"`
	c bool `oscript:",omitempty,public"`
}

func TestPublic(t *testing.T) {
	var p Public
	p.a = true

	b, err := Marshal(&p)
	require.Nil(t, err)
	want := `A<1,?,'A'=true,'B'=false>`
	assert.Equal(t, want, string(b))
}

// byte slices are special even if they're renamed types.
type renamedByte byte
type renamedByteSlice []byte
type renamedRenamedByteSlice []renamedByte

func TestEncodeRenamedByteSlice(t *testing.T) {
	s := renamedByteSlice("abc")
	result, err := Marshal(s)
	require.Nil(t, err)

	expect := `'YWJj'`
	assert.Equal(t, expect, string(result))

	r := renamedRenamedByteSlice("abc")
	result, err = Marshal(r)
	require.Nil(t, err)
	assert.Equal(t, expect, string(result))
}

var unsupportedValues = []interface{}{
	math.NaN(),
	math.Inf(-1),
	math.Inf(1),
}

func TestUnsupportedValues(t *testing.T) {
	for _, v := range unsupportedValues {
		if _, err := Marshal(v); err != nil {
			if _, ok := err.(*UnsupportedValueError); !ok {
				t.Errorf("for %v, got %T want UnsupportedValueError", v, err)
			}
		} else {
			t.Errorf("for %v, expected error", v)
		}
	}
}

// Ref has Marshaler and Unmarshaler methods with pointer receiver.
type Ref int

func (*Ref) MarshalOscript() ([]byte, error) {
	return []byte(`'ref'`), nil
}

func (r *Ref) UnmarshalOscript([]byte) error {
	*r = 12
	return nil
}

// Val has Marshaler methods with value receiver.
type Val int

func (Val) MarshalOscript() ([]byte, error) {
	return []byte(`'val'`), nil
}

func TestRefValMarshal(t *testing.T) {
	var s = struct {
		R0 Ref
		R1 *Ref
		V0 Val
		V1 *Val
	}{
		R0: 12,
		R1: new(Ref),
		V0: 13,
		V1: new(Val),
	}
	want := `A<1,?,'R0'='ref','R1'='ref','V0'='val','V1'='val'>`
	b, err := Marshal(&s)
	require.Nil(t, err)
	assert.Equal(t, want, string(b))
}

// C implements Marshaler and returns unescaped Oscript.
type C int

func (C) MarshalOscript() ([]byte, error) {
	return []byte(`'<&>'`), nil
}

func TestMarshalerEscaping(t *testing.T) {
	var c C
	want := `'<&>'`
	b, err := Marshal(c)
	require.Nil(t, err)
	assert.Equal(t, want, string(b))
}

func TestAnonymousFields(t *testing.T) {
	tests := []struct {
		label     string             // Test name
		makeInput func() interface{} // Function to create input value
		want      string             // Expected oscript output
	}{{
		// Unexported embedded field of non-struct type should not be serialized.
		label: "UnexportedEmbeddedInt",
		makeInput: func() interface{} {
			type (
				myInt int
				S     struct{ myInt }
			)
			return S{5}
		},
		want: `A<1,?>`,
	}, {
		// Exported embedded field of non-struct type should be serialized.
		label: "ExportedEmbeddedInt",
		makeInput: func() interface{} {
			type (
				MyInt int
				S     struct{ MyInt }
			)
			return S{5}
		},
		want: `A<1,?,'MyInt'=5>`,
	}, {
		// Unexported embedded field of pointer to non-struct type
		// should not be serialized.
		label: "UnexportedEmbeddedIntPointer",
		makeInput: func() interface{} {
			type (
				myInt int
				S     struct{ *myInt }
			)
			s := S{new(myInt)}
			*s.myInt = 5
			return s
		},
		want: `A<1,?>`,
	}, {
		// Exported embedded field of pointer to non-struct type
		// should be serialized.
		label: "ExportedEmbeddedIntPointer",
		makeInput: func() interface{} {
			type (
				MyInt int
				S     struct{ *MyInt }
			)
			s := S{new(MyInt)}
			*s.MyInt = 5
			return s
		},
		want: `A<1,?,'MyInt'=5>`,
	}, {
		// Exported fields of embedded structs should have their
		// exported fields be serialized regardless of whether the struct types
		// themselves are exported.
		label: "EmbeddedStruct",
		makeInput: func() interface{} {
			type (
				s1 struct{ x, X int }
				S2 struct{ y, Y int }
				S  struct {
					s1
					S2
				}
			)
			return S{s1{1, 2}, S2{3, 4}}
		},
		want: `A<1,?,'X'=2,'Y'=4>`,
	}, {
		// Exported fields of pointers to embedded structs should have their
		// exported fields be serialized regardless of whether the struct types
		// themselves are exported.
		label: "EmbeddedStructPointer",
		makeInput: func() interface{} {
			type (
				s1 struct{ x, X int }
				S2 struct{ y, Y int }
				S  struct {
					*s1
					*S2
				}
			)
			return S{&s1{1, 2}, &S2{3, 4}}
		},
		want: `A<1,?,'X'=2,'Y'=4>`,
	}, {
		// Exported fields on embedded unexported structs at multiple levels
		// of nesting should still be serialized.
		label: "NestedStructAndInts",
		makeInput: func() interface{} {
			type (
				MyInt1 int
				MyInt2 int
				myInt  int
				s2     struct {
					MyInt2
					myInt
				}
				s1 struct {
					MyInt1
					myInt
					s2
				}
				S struct {
					s1
					myInt
				}
			)
			return S{s1{1, 2, s2{3, 4}}, 6}
		},
		want: `A<1,?,'MyInt1'=1,'MyInt2'=3>`,
	}, {
		// If an anonymous struct pointer field is nil, we should ignore
		// the embedded fields behind it. Not properly doing so may
		// result in the wrong output or reflect panics.
		label: "EmbeddedFieldBehindNilPointer",
		makeInput: func() interface{} {
			type (
				S2 struct{ Field string }
				S  struct{ *S2 }
			)
			return S{}
		},
		want: `A<1,?>`,
	}}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			b, err := Marshal(tt.makeInput())
			require.Nil(t, err)
			require.Equal(t, tt.want, string(b))
		})
	}
}

type LevelA struct {
	S string
}

type LevelB struct {
	LevelA
	S string
}

type LevelC struct {
	S string
}

// Legal Go: We never use the repeated embedded field (S).
type LevelX struct {
	A int
	LevelA
	LevelB
}

// Even if a nil interface value is passed in
// as long as it implements MarshalOscript, it should be marshaled.
type nilMarshaler string

func (nm *nilMarshaler) MarshalOscript() ([]byte, error) {
	if nm == nil {
		return Marshal("0zenil0")
	}
	return Marshal("zenil:" + string(*nm))
}

func TestNilMarshal(t *testing.T) {
	testCases := []struct {
		v    interface{}
		want string
	}{
		{v: nil, want: `?`},
		{v: new(float64), want: `G0`},
		{v: []interface{}(nil), want: `?`},
		{v: []string(nil), want: `?`},
		{v: map[string]string(nil), want: `?`},
		{v: []byte(nil), want: `?`},
		{v: struct{ M string }{"gopher"}, want: `A<1,?,'M'='gopher'>`},
		{v: struct{ M Marshaler }{}, want: `A<1,?,'M'=?>`},
		{v: struct{ M Marshaler }{(*nilMarshaler)(nil)}, want: `A<1,?,'M'='0zenil0'>`},
		{v: struct{ M interface{} }{(*nilMarshaler)(nil)}, want: `A<1,?,'M'=?>`},
	}

	for _, tt := range testCases {
		got, err := Marshal(tt.v)
		require.Nil(t, err)
		assert.Equal(t, tt.want, string(got))
	}
}

func TestEmbedded(t *testing.T) {
	v := LevelB{
		LevelA{"A"},
		"B",
	}
	want := `A<1,?,'S'='B'>`
	b, err := Marshal(v)
	require.Nil(t, err)
	assert.Equal(t, want, string(b))

	// Now check that the duplicate field, S, does not appear.
	x := LevelX{
		A: 23,
	}
	want = `A<1,?,'A'=23>`
	b, err = Marshal(x)
	require.Nil(t, err)
	assert.Equal(t, want, string(b))
}

type LevelD struct { // Same as LevelA after tagging.
	XXX string `oscript:"S"`
}

// LevelD's tagged S field should dominate LevelA's.
type LevelY struct {
	LevelA
	LevelD
}

// Test that a field with a tag dominates untagged fields.
func TestTaggedFieldDominates(t *testing.T) {
	v := LevelY{
		LevelA{"LevelA"},
		LevelD{"LevelD"},
	}
	want := `A<1,?,'S'='LevelD'>`
	b, err := Marshal(v)
	require.Nil(t, err)
	assert.Equal(t, want, string(b))
}

// There are no tags here, so S should not appear.
type LevelZ struct {
	LevelA
	LevelC
	LevelY // Contains a tagged S field through LevelD; should not dominate.
}

func TestDuplicatedFieldDisappears(t *testing.T) {
	v := LevelZ{
		LevelA{"LevelA"},
		LevelC{"LevelC"},
		LevelY{
			LevelA{"nested LevelA"},
			LevelD{"nested LevelD"},
		},
	}
	want := `A<1,?>`
	b, err := Marshal(v)
	require.Nil(t, err)
	assert.Equal(t, want, string(b))
}

func TestStringBytes(t *testing.T) {
	t.Parallel()
	// Test that encodeState.stringBytes and encodeState.string use the same encoding.
	var r []rune
	for i := '\u0000'; i <= unicode.MaxRune; i++ {
		r = append(r, i)
	}
	s := string(r) + "\xff\xff\xffhello" // some invalid UTF-8 too
	es := &encodeState{}
	es.string(s)
	esBytes := &encodeState{}
	esBytes.stringBytes([]byte(s))
	enc := es.Buffer.String()
	encBytes := esBytes.Buffer.String()
	if enc != encBytes {
		i := 0
		for i < len(enc) && i < len(encBytes) && enc[i] == encBytes[i] {
			i++
		}

		enc = enc[i:]
		encBytes = encBytes[i:]
		i = 0

		for i < len(enc) && i < len(encBytes) && enc[len(enc)-i-1] == encBytes[len(encBytes)-i-1] {
			i++
		}
		enc = enc[:len(enc)-i]
		encBytes = encBytes[:len(encBytes)-i]

		if len(enc) > 20 {
			enc = enc[:20] + "..."
		}

		if len(encBytes) > 20 {
			encBytes = encBytes[:20] + "..."
		}
		t.Errorf("encodings differ at %#q vs %#q", enc, encBytes)
	}
}

var encodeStringTests = []struct {
	in   string
	want string
}{
	{"\x00", `'\u0000'`},
	{"\x01", `'\u0001'`},
	{"\x02", `'\u0002'`},
	{"\x03", `'\u0003'`},
	{"\x04", `'\u0004'`},
	{"\x05", `'\u0005'`},
	{"\x06", `'\u0006'`},
	{"\x07", `'\u0007'`},
	{"\x08", `'\u0008'`},
	{"\x09", `'\t'`},
	{"\x0a", `'\n'`},
	{"\x0b", `'\u000b'`},
	{"\x0c", `'\u000c'`},
	{"\x0d", `'\r'`},
	{"\x0e", `'\u000e'`},
	{"\x0f", `'\u000f'`},
	{"\x10", `'\u0010'`},
	{"\x11", `'\u0011'`},
	{"\x12", `'\u0012'`},
	{"\x13", `'\u0013'`},
	{"\x14", `'\u0014'`},
	{"\x15", `'\u0015'`},
	{"\x16", `'\u0016'`},
	{"\x17", `'\u0017'`},
	{"\x18", `'\u0018'`},
	{"\x19", `'\u0019'`},
	{"\x1a", `'\u001a'`},
	{"\x1b", `'\u001b'`},
	{"\x1c", `'\u001c'`},
	{"\x1d", `'\u001d'`},
	{"\x1e", `'\u001e'`},
	{"\x1f", `'\u001f'`},
}

func TestEncodeString(t *testing.T) {
	for _, tt := range encodeStringTests {
		b, err := Marshal(tt.in)

		if !assert.Nil(t, err) {
			continue
		}
		assert.Equal(t, tt.want, string(b))
	}
}

type oscriptbyte byte

func (b oscriptbyte) MarshalOscript() ([]byte, error) { return tenc(`A<1,?,'JB'=%d>`, b) }

type textbyte byte

func (b textbyte) MarshalText() ([]byte, error) { return tenc(`TB:%d`, b) }

type oscriptint int

func (i oscriptint) MarshalOscript() ([]byte, error) { return tenc(`A<1,?,'JI'=%d>`, i) }

type textint int

func (i textint) MarshalText() ([]byte, error) { return tenc(`TI:%d`, i) }

func tenc(format string, a ...interface{}) ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, format, a...)
	return buf.Bytes(), nil
}

func TestEncodeBytekind(t *testing.T) {
	testdata := []struct {
		data interface{}
		want string
	}{
		{byte(7), "7"},
		{oscriptbyte(7), `A<1,?,'JB'=7>`},
		{oscriptint(5), `A<1,?,'JI'=5>`},
		{[]byte{0, 1}, `'AAE='`},
		{[]oscriptbyte{0, 1}, `{A<1,?,'JB'=0>,A<1,?,'JB'=1>}`},
		{[][]oscriptbyte{{0, 1}, {3}}, `{{A<1,?,'JB'=0>,A<1,?,'JB'=1>},{A<1,?,'JB'=3>}}`},
		{[]oscriptint{5, 4}, `{A<1,?,'JI'=5>,A<1,?,'JI'=4>}`},
		{[]int{9, 3}, `{9,3}`},
	}

	for _, tt := range testdata {
		b, err := Marshal(tt.data)
		if !assert.Nil(t, err) {
			continue
		}
		assert.Equal(t, tt.want, string(b))
	}
}

func TestTextMarshalerMapKeysAreSorted(t *testing.T) {
	want := `A<1,?,'a:z'=3,'x:y'=1,'y:x'=2,'z:a'=4>`
	b, err := Marshal(map[unmarshalerText]int{
		{"x", "y"}: 1,
		{"y", "x"}: 2,
		{"a", "z"}: 3,
		{"z", "a"}: 4,
	})

	require.Nil(t, err)
	assert.Equal(t, want, string(b))
}

func TestFloat64(t *testing.T) {
	want := "G1.7976931348623157e+308"
	b, err := Marshal(math.MaxFloat64)

	require.Nil(t, err)
	assert.Equal(t, want, string(b))
}

func TestTime(t *testing.T) {
	testdata := []struct {
		data time.Time
		want string
	}{
		{data: time.Date(2017, 12, 4, 11, 47, 16, 0, time.UTC), want: "D/2017/12/4:11:47:16"},
		{data: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), want: "D/1/1/1:0:0:0"},
	}

	for _, tt := range testdata {
		b, err := Marshal(tt.data)
		require.Nil(t, err)
		assert.Equal(t, tt.want, string(b))
	}

}

type marshalBuf struct{}

func (an marshalBuf) MarshalOscriptBuf(buf Buffer) error {
	buf.WriteString("A<1,N")
	buf.WriteByte(',')
	buf.WriteStringValue("key")
	buf.WriteByte('=')
	buf.WriteEncode(struct{ A string }{A: "B"})
	buf.WriteByte('>')

	return nil
}

func TestMarshalerBuf(t *testing.T) {
	want := "A<1,N,'key'=A<1,?,'A'='B'>>"
	b, err := Marshal(marshalBuf{})

	require.Nil(t, err)
	assert.Equal(t, want, string(b))
}

func TestError(t *testing.T) {
	e := Error(1024)
	want := "E1024"
	b, err := Marshal(e)

	require.Nil(t, err)
	assert.Equal(t, want, string(b))
}

type sdoNameTag struct {
	Name SDOName `oscript:"world.gopher"`
	Age  int
}

type sdoUnexported struct {
	name SDOName `oscript:"world.gopher,public"`
}

func TestSDOName(t *testing.T) {
	testdata := []struct {
		data interface{}
		want string
		err  *MarshalerError
	}{
		{data: sdoNameTag{Age: 6}, want: "A<1,?,'_SDOName'='world.gopher','Age'=6>"},
		{data: sdoUnexported{}, want: "A<1,?,'_SDOName'='world.gopher'>"},
	}

	for _, tt := range testdata {
		b, err := Marshal(tt.data)

		if tt.err != nil {
			assert.Equal(t, tt.err, err)
			continue
		}

		if !assert.Nil(t, err) {
			continue
		}
		assert.Equal(t, tt.want, string(b))
	}
}

type marshalPanic struct{}

func (marshalPanic) MarshalOscript() ([]byte, error) { panic(0xdead) }

func TestMarshalPanic(t *testing.T) {
	defer func() {
		got := recover()
		assert.Equal(t, 0xdead, got)
	}()

	Marshal(&marshalPanic{})
	t.Error("Marshal should have panicked")
}

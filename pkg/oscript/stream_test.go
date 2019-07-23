package oscript

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"reflect"
	"strings"
	"testing"
)

// Test values for the stream test.
// One of each Oscript kind.
var streamTest = []interface{}{
	0.1,
	"hello",
	nil,
	true,
	false,
	[]interface{}{"a", "b", "c"},
	map[string]interface{}{"K": "Kelvin", "ß": "long s"},
	3.14, // another value to make sure something can follow map
}

var streamEncoded = `G0.1
'hello'
?
true
false
{'a','b','c'}
A<1,?,'ß'='long s','K'='Kelvin'>
G3.14
`

func TestEncoder(t *testing.T) {
	for i := 0; i <= len(streamTest); i++ {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		enc.EnableEncodeNL()
		// Error that enc.SetIndent("", "") turns off indentation.
		//enc.SetIndent(">", ".")
		//enc.SetIndent("", "")
		for j, v := range streamTest[0:i] {
			if err := enc.Encode(v); err != nil {
				t.Fatalf("encode #%d: %v", j, err)
			}
		}

		if have, want := buf.String(), nlines(streamEncoded, i); have != want {
			t.Errorf("encoding %d items: mismatch", i)
			diff(t, []byte(have), []byte(want))
			break
		}
	}
}

var streamDecoded = `G0.1
'hello'
?
true
false
{'a','b','c'}
A<1,?,'ß'='long s','K'='Kelvin'>
G3.14
`

func TestDecoder(t *testing.T) {
	for i := 0; i <= len(streamTest); i++ {
		// Use stream without newlines as input,
		// just to stress the decoder even more.
		// Our test input does not include back-to-back numbers.
		// Otherwise stripping the newlines would
		// merge two adjacent Oscript values.
		var buf bytes.Buffer
		for _, c := range nlines(streamDecoded, i) {
			if c != '\n' {
				buf.WriteRune(c)
			}
		}
		out := make([]interface{}, i)
		dec := NewDecoder(&buf)
		for j := range out {
			if err := dec.Decode(&out[j]); err != nil {
				t.Fatalf("decode #%d/%d: %v", j, i, err)
			}
		}

		if !reflect.DeepEqual(out, streamTest[0:i]) {
			t.Errorf("decoding %d items: mismatch", i)
			for j := range out {
				if !reflect.DeepEqual(out[j], streamTest[j]) {
					t.Errorf("#%d: have %v want %v", j, out[j], streamTest[j])
				}
			}
			break
		}
	}
}

func TestDecoderBuffered(t *testing.T) {
	r := strings.NewReader(`A<1,?,'Name'= 'Gopher'> extra `)
	var m struct {
		Name string
	}

	d := NewDecoder(r)
	err := d.Decode(&m)
	if err != nil {
		t.Fatal(err)
	}

	if m.Name != "Gopher" {
		t.Errorf("Name = %q; want Gopher", m.Name)
	}
	rest, err := ioutil.ReadAll(d.Buffered())

	if err != nil {
		t.Fatal(err)
	}
	if g, w := string(rest), " extra "; g != w {
		t.Errorf("Remaining = %q; want %q", g, w)
	}
}

func nlines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	for i, c := range s {
		if c == '\n' {
			if n--; n == 0 {
				return s[0 : i+1]
			}
		}
	}
	return s
}

var blockingTests = []string{
	`A<1,?,'x'= 1>`,
	`{1, 2, 3}`,
}

func TestBlocking(t *testing.T) {
	for _, enc := range blockingTests {
		r, w := net.Pipe()
		go w.Write([]byte(enc))
		var val interface{}

		// If Decode reads beyond what w.Write writes above,
		// it will block, and the test will deadlock.
		if err := NewDecoder(r).Decode(&val); err != nil {
			t.Errorf("decoding %s: %v", enc, err)
		}
		r.Close()
		w.Close()
	}
}

func BenchmarkEncoderEncode(b *testing.B) {
	b.ReportAllocs()
	type T struct {
		X, Y string
	}
	v := &T{"foo", "bar"}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := NewEncoder(ioutil.Discard).Encode(v); err != nil {
				b.Fatal(err)
			}
		}
	})
}

type tokenStreamCase struct {
	oscript string

	expTokens []interface{}
}

type decodeThis struct {
	v interface{}
}

var tokenStreamCases = []tokenStreamCase{
	// streaming token cases
	{oscript: `10`, expTokens: []interface{}{int64(10)}},
	{oscript: ` {10} `, expTokens: []interface{}{
		Delim('{'), int64(10), Delim('}')}},

	{oscript: ` {false,10,'b'} `, expTokens: []interface{}{
		Delim('{'), false, int64(10), "b", Delim('}')}},

	{oscript: `A<1,?, 'a'= 1 >`, expTokens: []interface{}{
		Delim('A'), "a", int64(1), Delim('>')}},

	{oscript: `A<1,?,'a'= 1, 'b'='3'>`, expTokens: []interface{}{
		Delim('A'), "a", int64(1), "b", "3", Delim('>')}},

	{oscript: ` {A<1,?,'a'= 1>, A<1,?,'a'= 2>} `, expTokens: []interface{}{
		Delim('{'),
		Delim('A'), "a", int64(1), Delim('>'),
		Delim('A'), "a", int64(2), Delim('>'),
		Delim('}')}},

	{oscript: `A<1,?,'obj'= A<1,?,'a'= 1>>`, expTokens: []interface{}{
		Delim('A'), "obj",
		Delim('A'), "a", int64(1),
		Delim('>'), Delim('>'),
	}},

	{oscript: `A<1,?,'obj'= {A<1,?,'a'= 1>}>`, expTokens: []interface{}{
		Delim('A'), "obj", Delim('{'),
		Delim('A'), "a", int64(1), Delim('>'),
		Delim('}'), Delim('>')}},

	// streaming tokens with intermittent Decode()
	{oscript: `A<1,?, 'a'= 1 >`, expTokens: []interface{}{
		Delim('A'), "a",
		decodeThis{int64(1)},
		Delim('>')}},

	{oscript: ` { A<1,?, 'a' = 1 > } `, expTokens: []interface{}{
		Delim('{'),
		decodeThis{map[string]interface{}{"a": int64(1)}},
		Delim('}')}},

	{oscript: ` {A<1,?,'a'= 1>,A<1,?,'a'= 2>} `, expTokens: []interface{}{
		Delim('{'),
		decodeThis{map[string]interface{}{"a": int64(1)}},
		decodeThis{map[string]interface{}{"a": int64(2)}},
		Delim('}')}},

	{oscript: `A<1,?, 'obj' = { A<1,?, 'a' = 1 > } >`, expTokens: []interface{}{
		Delim('A'), "obj", Delim('{'),
		decodeThis{map[string]interface{}{"a": int64(1)}},
		Delim('}'), Delim('>')}},

	{oscript: `A<1,?, 'obj'= A<1,?,'a'= 1>>`, expTokens: []interface{}{
		Delim('A'), "obj",
		decodeThis{map[string]interface{}{"a": int64(1)}},
		Delim('>')}},

	{oscript: `A<1,?, 'obj'= {A<1,?,'a'= 1>}>`, expTokens: []interface{}{
		Delim('A'), "obj",
		decodeThis{[]interface{}{
			map[string]interface{}{"a": int64(1)},
		}},
		Delim('>')}},

	{oscript: ` {A<1,?,'a'= 1> A<1,?,'a'= 2>} `, expTokens: []interface{}{
		Delim('{'),
		decodeThis{map[string]interface{}{"a": int64(1)}},
		decodeThis{&SyntaxError{"expected comma after array element", 16}},
	}},

	{oscript: `A<1,?, '` + strings.Repeat("a", 513) + `' 1 >`, expTokens: []interface{}{
		Delim('A'), strings.Repeat("a", 513),
		decodeThis{&SyntaxError{"expected equally after object key", 523}},
	}},

	{oscript: `A<1,?, '\a' >`, expTokens: []interface{}{
		Delim('A'),
		&SyntaxError{"invalid character 'a' in string escape code", 3},
	}},

	{oscript: ` \a`, expTokens: []interface{}{
		&SyntaxError{"invalid character '\\\\' looking for beginning of value", 1},
	}},
}

func TestDecodeInStream(t *testing.T) {
	for ci, tcase := range tokenStreamCases {
		dec := NewDecoder(strings.NewReader(tcase.oscript))
		for i, etk := range tcase.expTokens {

			var tk interface{}
			var err error

			if dt, ok := etk.(decodeThis); ok {
				etk = dt.v
				err = dec.Decode(&tk)
			} else {
				tk, err = dec.Token()
			}

			if experr, ok := etk.(error); ok {
				if err == nil || !reflect.DeepEqual(err, experr) {
					t.Errorf("case %v: Expected error %#v in %q, but was %#v", ci, experr, tcase.oscript, err)
				}
				break

			} else if err == io.EOF {
				t.Errorf("case %v: Unexpected EOF in %q", ci, tcase.oscript)
				break

			} else if err != nil {
				t.Errorf("case %v: Unexpected error '%#v' in %q", ci, err, tcase.oscript)
				break
			}

			if !reflect.DeepEqual(tk, etk) {
				t.Errorf(`case %v: %q @ %v expected %T(%v) was %T(%v)`, ci, tcase.oscript, i, etk, etk, tk, tk)
				break
			}
		}
	}
}

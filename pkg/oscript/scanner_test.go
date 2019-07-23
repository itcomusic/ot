package oscript

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"testing"
)

var validTests = []struct {
	data string
	ok   bool
}{
	{`foo`, false},
	{`}{`, false},
	{`{]`, false},
	{`{}`, true},
	{`A<1,?,'foo'='bar'>`, true},
	{`A<1,?,'foo'='bar','bar'=A<1,N,'baz'={'qux'}>>`, true},
}

func TestValid(t *testing.T) {
	for _, tt := range validTests {
		if ok := Valid([]byte(tt.data)); ok != tt.ok {
			t.Errorf("Valid(%#q) = %v, want %v", tt.data, ok, tt.ok)
		}
	}
}

// Tests of simple examples.
type example struct {
	compact string
	indent  string
}

var examples = []example{
	{`1`, `1`},
	{`A<1,?>`, `A<1,?>`},
	{`{}`, `{}`},
	{`A<1,?,''=2>`, "A<1,?,\n\t''= 2\n>"},
	{`{3}`, "{\n\t3\n}"},
	{`{1,2,3}`, "{\n\t1,\n\t2,\n\t3\n}"},
	{`A<1,?,'x'=1>`, "A<1,?,\n\t'x'= 1\n>"},
	{ex1, ex1i},
}
var ex1 = `{true,false,?,'x',1,G1.5,0,G-5e+2}`
var ex1i = `{
	true,
	false,
	?,
	'x',
	1,
	G1.5,
	0,
	G-5e+2
}`

func TestCompact(t *testing.T) {
	var buf bytes.Buffer
	for _, tt := range examples {
		buf.Reset()
		if err := Compact(&buf, []byte(tt.compact)); err != nil {
			t.Errorf("Compact(%#q): %v", tt.compact, err)
		} else if s := buf.String(); s != tt.compact {
			t.Errorf("Compact(%#q) = %#q, want original", tt.compact, s)
		}
		buf.Reset()
		if err := Compact(&buf, []byte(tt.indent)); err != nil {
			t.Errorf("Compact(%#q): %v", tt.indent, err)
			continue
		} else if s := buf.String(); s != tt.compact {
			t.Errorf("Compact(%#q) = %#q, want %#q", tt.indent, s, tt.compact)
		}
	}
}

func TestCompactSeparators(t *testing.T) {
	// U+2028 and U+2029 should be escaped inside strings.
	// They should not appear outside strings.
	tests := []struct {
		in, compact string
	}{
		{"A<1,?,'\u2028'= 1>", `A<1,?,'\u2028'=1>`},
		{"A<1,?,'\u2029' =2>", `A<1,?,'\u2029'=2>`},
	}
	for _, tt := range tests {
		var buf bytes.Buffer
		if err := Compact(&buf, []byte(tt.in)); err != nil {
			t.Errorf("Compact(%q): %v", tt.in, err)
		} else if s := buf.String(); s != tt.compact {
			t.Errorf("Compact(%q) = %q, want %q", tt.in, s, tt.compact)
		}
	}
}

/*func TestIndent(t *testing.T) {
	var buf bytes.Buffer
	for _, tt := range examples {
		buf.Reset()
		if err := Indent(&buf, []byte(tt.indent), "", "\t"); err != nil {
			t.Errorf("Indent(%#q): %v", tt.indent, err)
		} else if s := buf.String(); s != tt.indent {
			t.Errorf("Indent(%#q) = %#q, want original", tt.indent, s)
		}
		buf.Reset()
		if err := Indent(&buf, []byte(tt.compact), "", "\t"); err != nil {
			t.Errorf("Indent(%#q): %v", tt.compact, err)
			continue
		} else if s := buf.String(); s != tt.indent {
			t.Errorf("Indent(%#q) = %#q, want %#q", tt.compact, s, tt.indent)
		}
	}
}*/

// Tests of a large random structure.
func TestCompactBig(t *testing.T) {
	initBig()
	var buf bytes.Buffer
	if err := Compact(&buf, oscriptBig); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	b := buf.Bytes()
	if !bytes.Equal(b, oscriptBig) {
		t.Error("Compact(oscriptBig) != oscriptBig")
		diff(t, b, oscriptBig)
		return
	}
}

func diff(t *testing.T, a, b []byte) {
	for i := 0; ; i++ {
		if i >= len(a) || i >= len(b) || a[i] != b[i] {
			j := i - 10
			if j < 0 {
				j = 0
			}
			t.Errorf("diverge at %d: «%s» vs «%s»", i, trim(a[j:]), trim(b[j:]))
			return
		}
	}
}

func trim(b []byte) []byte {
	if len(b) > 20 {
		return b[0:20]
	}
	return b
}

// Generate a random oscript object.
var oscriptBig []byte

func initBig() {
	n := 10000
	if testing.Short() {
		n = 100
	}
	b, err := Marshal(genValue(n))
	if err != nil {
		panic(err)
	}
	oscriptBig = b
}

func genValue(n int) interface{} {
	if n > 1 {
		switch rand.Intn(2) {
		case 0:
			return genArray(n)
		case 1:
			return genMap(n)
		}
	}
	switch rand.Intn(3) {
	case 0:
		return rand.Intn(2) == 0
	case 1:
		return "G" + fmt.Sprint(rand.NormFloat64())
	case 2:
		return genString(30)
	}
	panic("unreachable")
}

func genString(stddev float64) string {
	n := int(math.Abs(rand.NormFloat64()*stddev + stddev/2))
	c := make([]rune, n)
	for i := range c {
		f := math.Abs(rand.NormFloat64()*64 + 32)
		if f > 0x10ffff {
			f = 0x10ffff
		}
		c[i] = rune(f)
	}
	return string(c)
}

func genArray(n int) []interface{} {
	f := int(math.Abs(rand.NormFloat64()) * math.Min(10, float64(n/2)))
	if f > n {
		f = n
	}
	if f < 1 {
		f = 1
	}
	x := make([]interface{}, f)
	for i := range x {
		x[i] = genValue(((i+1)*n)/f - (i*n)/f)
	}
	return x
}

func genMap(n int) map[string]interface{} {
	f := int(math.Abs(rand.NormFloat64()) * math.Min(10, float64(n/2)))
	if f > n {
		f = n
	}
	if n > 0 && f == 0 {
		f = 1
	}
	x := make(map[string]interface{})
	for i := 0; i < f; i++ {
		x[genString(10)] = genValue(((i+1)*n)/f - (i*n)/f)
	}
	return x
}

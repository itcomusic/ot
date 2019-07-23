package oscript

import (
	"bytes"
	"encoding"
	"encoding/base64"
	"errors"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// M is a convenient alias for a map[string]interface{} map, useful for short define
// For instance: script.M{"key1": 1, "key2": true}
//
// There's no special handling for this type in addition to what's done anyway
// for an equivalent map type.
type M map[string]interface{}

// Marshal returns the oscript encoding of v.
//
// Marshal traverses the value v recursively.
// If an encountered value implements the Marshaler interface
// and is not a nil pointer, Marshal calls its MarshalOscript method
// to produce oscript. If no MarshalOscript method is present but the
// value implements MarshalerBuf instead, Marshal calls
// its MarshalOscriptBuf method and encodes the result without compact
// and validation oscript value.
// The nil pointer exception is not strictly necessary
// but mimics a similar, necessary exception in the behavior of
// UnmarshalOscript.
//
// Otherwise, Marshal uses the following type-dependent default encodings:
//
// Nil values encode as oscript undefined: ?.
//
// Boolean values encode as oscript booleans: true, false
//
// Floating point, integer values encode as oscript numbers: G1.1 ; 1.
//
// String values encode as oscript strings coerced to valid UTF-8: 'text'.
//
// Time values encode as oscript date: D/2006/1/2:15:4:5.
//
// Array and slice values encode as oscript arrays: {...}, except that
// []byte encodes as a base64-encoded string, and a nil slice
// encodes as the undefined Oscript undefined value.
//
// Struct values and map encode as oscript objects: A<1,?,'key'=value>.
// Each exported struct field becomes a member of the object, using the
// field name as the object key, unless the field is omitted for one of the
// reasons given below.
//
// SDOName encode as oscript service data object: '_SDOName'='namespace.Name'.
// This type may be private field in a struct.

// The encoding of each struct field can be customized by the format string
// stored under the "oscript" key in the struct field's tag.
// The format string gives the name of the field, possibly followed by a
// comma-separated list of options. The name may be empty in order to
// specify options without overriding the default field name.
//
// The "omitempty" option specifies that the field should be omitted
// from the encoding if the field has an empty value, defined as
// false, 0, a nil pointer, a nil interface value, and any empty array,
// slice, map, or string.
//
// As a special case, if the field tag is "-", the field is always omitted.
// Note that a field with name "-" can still be generated using the tag "-,".
//
// Anonymous struct fields are usually marshaled as if their inner exported fields
// were fields in the outer struct, subject to the usual Go visibility rules amended
// as described in the next paragraph.
// An anonymous struct field with a name given in its Oscript tag is treated as
// having that name, rather than being anonymous.
// An anonymous struct field of interface type is treated the same as having
// that type as its name, rather than being anonymous.
//
// The Go visibility rules for struct fields are amended for Oscript when
// deciding which field to marshal or unmarshal. If there are
// multiple fields at the same level, and that level is the least
// nested (and would therefore be the nesting level selected by the
// usual Go rules), the following extra rules apply:
//
// 1) Of those fields, if any are Oscript-tagged, only tagged fields are considered,
// even if there are multiple untagged fields that would otherwise conflict.
//
// 2) If there is exactly one field (tagged or not according to the first rule), that is selected.
//
// 3) Otherwise there are multiple fields, and all are ignored; no error occurs.
//
// Map values encode as oscript objects: A<1,?,'key'=value>. The map's key type must either be a
// string, an integer type, or implement encoding.TextMarshaler.
//   - string keys are used directly
//   - encoding.TextMarshalers are marshaled
//   - integer keys are converted to strings
//
// Pointer values encode as the value pointed to.
// A nil pointer encodes as the undefined Oscript value.
//
// Interface values encode as the value contained in the interface.
// A nil interface value encodes as the undefined Oscript value.
//
// Channel, complex, and function values cannot be encoded in oscript.
// Attempting to encode such a value causes Marshal to return
// an UnsupportedTypeError.
//
// Oscript cannot represent cyclic data structures and Marshal does not
// handle them. Passing cyclic structures to Marshal will result in
// an infinite recursion.
//
func Marshal(v interface{}) ([]byte, error) {
	e := newEncodeState()

	err := e.marshal(v)
	if err != nil {
		return nil, err
	}
	buf := append([]byte(nil), e.Bytes()...)

	e.Reset()
	encodeStatePool.Put(e)

	return buf, nil
}

// Marshaler is the interface implemented by types that can marshal themselves into valid oscript.
type Marshaler interface {
	MarshalOscript() ([]byte, error)
}

// MarshalerBuf is the interface implemented by types that can marshal themselves and does not
// require addition allocation as a Marshaller for returning []byte.
type MarshalerBuf interface {
	MarshalOscriptBuf(buf Buffer) error
}

// Buffer is the interface implemented by internal encodeState.
type Buffer interface {
	WriteString(s string) (int, error)
	WriteStringValue(s string)
	WriteByte(b byte) error
	WriteEncode(v interface{})
}

// An UnsupportedTypeError is returned by Marshal when attempting
// to encode an unsupported value type.
type UnsupportedTypeError struct {
	Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return "oscript: unsupported type: " + e.Type.String()
}

// An UnsupportedValueError is returned by Marshal when attempting
// to encode an unsupported value.
type UnsupportedValueError struct {
	Value reflect.Value
	Str   string
}

func (e *UnsupportedValueError) Error() string {
	return "oscript: unsupported value: " + e.Str
}

type MarshalerError struct {
	Type reflect.Type
	Err  error
}

func (e *MarshalerError) Error() string {
	return "oscript: error calling MarshalOscript for type " + e.Type.String() + ": " + e.Err.Error()
}

var hex = "0123456789abcdef"

// An encodeState encodes Oscript into a bytes.Buffer.
type encodeState struct {
	bytes.Buffer // accumulated output
	scratch      [64]byte
}

var encodeStatePool sync.Pool

func newEncodeState() *encodeState {
	if v := encodeStatePool.Get(); v != nil {
		e := v.(*encodeState)
		e.Reset()
		return e
	}
	return new(encodeState)
}

// oscriptError is an error wrapper type for internal use only.
// Panics with errors are wrapped in oscriptError so that the top-level recover
// can distinguish intentional panics from this package.
type oscriptError struct{ error }

func (e *encodeState) marshal(v interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if oe, ok := r.(oscriptError); ok {
				err = oe.error
			} else {
				panic(r)
			}
		}
	}()
	e.reflectValue(reflect.ValueOf(v))
	return nil
}

func (e *encodeState) error(err error) {
	panic(oscriptError{err})
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

func (e *encodeState) reflectValue(v reflect.Value) {
	valueEncoder(v)(e, v)
}

type encoderFunc func(e *encodeState, v reflect.Value)

var encoderCache sync.Map // map[reflect.Type]encoderFunc

func valueEncoder(v reflect.Value) encoderFunc {
	if !v.IsValid() {
		return invalidValueEncoder
	}
	return typeEncoder(v.Type())
}

func typeEncoder(t reflect.Type) encoderFunc {
	if fi, ok := encoderCache.Load(t); ok {
		return fi.(encoderFunc)
	}

	// to deal with recursive types, populate the map with an
	// indirect func before we build it. This type waits on the
	// real func (f) to be ready and then calls it. This indirect
	// func is only used for recursive types
	var (
		wg sync.WaitGroup
		f  encoderFunc
	)
	wg.Add(1)
	fi, loaded := encoderCache.LoadOrStore(t, encoderFunc(func(e *encodeState, v reflect.Value) {
		wg.Wait()
		f(e, v)
	}))

	if loaded {
		return fi.(encoderFunc)
	}

	// compute the real encoder and replace the indirect func with it
	f = newTypeEncoder(t, true)
	wg.Done()
	encoderCache.Store(t, f)
	return f
}

var (
	marshalerType     = reflect.TypeOf(new(Marshaler)).Elem()
	marshalerBufType  = reflect.TypeOf(new(MarshalerBuf)).Elem()
	textMarshalerType = reflect.TypeOf(new(encoding.TextMarshaler)).Elem()
	timeType          = reflect.TypeOf(time.Time{})
	sdoType           = reflect.TypeOf(SDOName{})

	undefined = byte('?')
)

// newTypeEncoder constructs an encoderFunc for a type.
// The returned encoder only checks CanAddr when allowAddr is true.
func newTypeEncoder(t reflect.Type, allowAddr bool) encoderFunc {
	if t.Implements(marshalerBufType) {
		return marshalerBufEncoder
	}
	if t.Kind() != reflect.Ptr && allowAddr {
		if reflect.PtrTo(t).Implements(marshalerBufType) {
			return newCondAddrEncoder(addrMarshalerBufEncoder, newTypeEncoder(t, false))
		}
	}

	if t.Implements(marshalerType) {
		return marshalerEncoder
	}
	if t.Kind() != reflect.Ptr && allowAddr {
		if reflect.PtrTo(t).Implements(marshalerType) {
			return newCondAddrEncoder(addrMarshalerEncoder, newTypeEncoder(t, false))
		}
	}

	switch t.Kind() {
	case reflect.Bool:
		return boolEncoder
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intEncoder
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uintEncoder
	case reflect.Float32:
		return float32Encoder
	case reflect.Float64:
		return float64Encoder
	case reflect.String:
		return stringEncoder
	case reflect.Interface:
		return interfaceEncoder
	case reflect.Struct:
		if t == timeType {
			return timeEncoder
		}
		return newStructEncoder(t)
	case reflect.Map:
		return newMapEncoder(t)
	case reflect.Slice:
		return newSliceEncoder(t)
	case reflect.Array:
		return newArrayEncoder(t)
	case reflect.Ptr:
		return newPtrEncoder(t)
	default:
		return unsupportedTypeEncoder
	}
}

func invalidValueEncoder(e *encodeState, v reflect.Value) {
	e.WriteByte(undefined)
}

func marshalerBufEncoder(e *encodeState, v reflect.Value) {
	if v.Kind() == reflect.Ptr && v.IsNil() {
		e.WriteByte(undefined)
		return
	}

	m, ok := v.Interface().(MarshalerBuf)
	if !ok {
		e.WriteByte(undefined)
		return
	}

	if err := m.MarshalOscriptBuf(e); err != nil {
		e.error(&MarshalerError{v.Type(), err})
	}
}

func addrMarshalerBufEncoder(e *encodeState, v reflect.Value) {
	va := v.Addr()
	if va.IsNil() {
		e.WriteByte(undefined)
		return
	}

	m, ok := v.Interface().(MarshalerBuf)
	if !ok {
		e.WriteByte(undefined)
		return
	}

	if err := m.MarshalOscriptBuf(e); err != nil {
		e.error(&MarshalerError{v.Type(), err})
	}
}

func marshalerEncoder(e *encodeState, v reflect.Value) {
	if v.Kind() == reflect.Ptr && v.IsNil() {
		e.WriteByte(undefined)
		return
	}

	m, ok := v.Interface().(Marshaler)
	if !ok {
		e.WriteByte(undefined)
		return
	}

	b, err := m.MarshalOscript()
	if err == nil {
		err = compact(&e.Buffer, b)
	}

	if err != nil {
		e.error(&MarshalerError{v.Type(), err})
	}
}

func addrMarshalerEncoder(e *encodeState, v reflect.Value) {
	va := v.Addr()
	if va.IsNil() {
		e.WriteByte(undefined)
		return
	}

	m := va.Interface().(Marshaler)
	b, err := m.MarshalOscript()

	if err == nil {
		// copy oscript into buffer, checking validity
		err = compact(&e.Buffer, b)
	}

	if err != nil {
		e.error(&MarshalerError{v.Type(), err})
	}
}

func boolEncoder(e *encodeState, v reflect.Value) {
	if v.Bool() {
		e.WriteString("true")
	} else {
		e.WriteString("false")
	}
}

func intEncoder(e *encodeState, v reflect.Value) {
	b := strconv.AppendInt(e.scratch[:0], v.Int(), 10)
	e.Write(b)
}

func uintEncoder(e *encodeState, v reflect.Value) {
	b := strconv.AppendUint(e.scratch[:0], v.Uint(), 10)
	e.Write(b)
}

type floatEncoder int // number of bits
func (bits floatEncoder) encode(e *encodeState, v reflect.Value) {
	f := v.Float()
	if math.IsInf(f, 0) || math.IsNaN(f) {
		e.error(&UnsupportedValueError{v, strconv.FormatFloat(f, 'g', -1, int(bits))})
	}
	b := e.scratch[:0]
	abs := math.Abs(f)
	fmt := byte('f')
	// Note: Must use float32 comparisons for underlying float32 value to get precise cutoffs right.
	if abs != 0 {
		if bits == 64 && (abs < 1e-6 || abs >= 1e21) || bits == 32 && (float32(abs) < 1e-6 || float32(abs) >= 1e21) {
			fmt = 'e'
		}
	}
	b = strconv.AppendFloat(b, f, fmt, -1, int(bits))
	if fmt == 'e' {
		// clean up e-09 to e-9
		n := len(b)
		if n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
			b[n-2] = b[n-1]
			b = b[:n-1]
		}
	}
	e.WriteByte('G')
	e.Write(b)
}

var (
	float32Encoder = (floatEncoder(32)).encode
	float64Encoder = (floatEncoder(64)).encode
)

func stringEncoder(e *encodeState, v reflect.Value) {
	e.string(v.String())
}

func interfaceEncoder(e *encodeState, v reflect.Value) {
	if v.IsNil() {
		e.WriteByte(undefined)
		return
	}
	e.reflectValue(v.Elem())
}

func unsupportedTypeEncoder(e *encodeState, v reflect.Value) {
	e.error(&UnsupportedTypeError{v.Type()})
}

type structEncoder struct {
	fields []field
}

func (se *structEncoder) encode(e *encodeState, v reflect.Value) {
	e.WriteString("A<1,?")

FieldLoop:
	for i := range se.fields {
		f := &se.fields[i]
		fv := v

		// find the nested struct field by following f.index
		for _, i := range f.index {
			if fv.Kind() == reflect.Ptr {
				if fv.IsNil() {
					continue FieldLoop
				}
				fv = fv.Elem()
			}
			fv = fv.Field(i)
		}

		if f.omitEmpty && isEmptyValue(fv) {
			continue
		}
		e.WriteByte(',')
		e.string(f.name)
		e.WriteByte('=')
		f.encoder(e, fv)
	}

	e.WriteByte('>')
}

func newStructEncoder(t reflect.Type) encoderFunc {
	se := structEncoder{fields: cachedTypeFields(t)}
	return se.encode
}

type mapEncoder struct {
	elemEnc encoderFunc
}

func (me *mapEncoder) encode(e *encodeState, v reflect.Value) {
	if v.IsNil() {
		e.WriteByte(undefined)
		return
	}

	e.WriteString("A<1,?")
	// extract and sort the keys
	keys := v.MapKeys()
	sv := make([]reflectWithString, len(keys))
	for i, v := range keys {
		sv[i].v = v
		if err := sv[i].resolve(); err != nil {
			e.error(&MarshalerError{v.Type(), err})
		}
	}

	sort.Slice(sv, func(i, j int) bool { return sv[i].s < sv[j].s })
	for _, kv := range sv {
		e.WriteByte(',')
		e.string(kv.s)
		e.WriteByte('=')
		me.elemEnc(e, v.MapIndex(kv.v))
	}
	e.WriteByte('>')
}

func newMapEncoder(t reflect.Type) encoderFunc {
	switch t.Key().Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
	default:
		if !t.Key().Implements(textMarshalerType) {
			return unsupportedTypeEncoder
		}
	}
	me := mapEncoder{typeEncoder(t.Elem())}
	return me.encode
}

// sliceEncoder just wraps an arrayEncoder, checking to make sure the value isn't nil.
type sliceEncoder struct {
	arrayEnc encoderFunc
}

func (se *sliceEncoder) encode(e *encodeState, v reflect.Value) {
	if v.IsNil() {
		e.WriteByte(undefined)
		return
	}
	se.arrayEnc(e, v)
}

func encodeByteSlice(e *encodeState, v reflect.Value) {
	if v.IsNil() {
		e.WriteByte(undefined)
		return
	}
	s := v.Bytes()
	e.WriteByte('\'')
	if len(s) < 1024 {
		// for small buffers, using Encode directly is much faster.
		dst := make([]byte, base64.StdEncoding.EncodedLen(len(s)))
		base64.StdEncoding.Encode(dst, s)
		e.Write(dst)
	} else {
		// for large buffers, avoid unnecessary extra temporary
		// buffer space.
		enc := base64.NewEncoder(base64.StdEncoding, e)
		enc.Write(s)
		enc.Close()
	}
	e.WriteByte('\'')
}

func newSliceEncoder(t reflect.Type) encoderFunc {
	// Byte slices get special treatment; arrays don't.
	if t.Elem().Kind() == reflect.Uint8 {
		p := reflect.PtrTo(t.Elem())
		if !p.Implements(marshalerType) && !p.Implements(textMarshalerType) {
			return encodeByteSlice
		}
	}
	enc := sliceEncoder{newArrayEncoder(t)}
	return enc.encode
}

type arrayEncoder struct {
	elemEnc encoderFunc
}

func (ae *arrayEncoder) encode(e *encodeState, v reflect.Value) {
	e.WriteByte('{')
	n := v.Len()
	for i := 0; i < n; i++ {
		if i > 0 {
			e.WriteByte(',')
		}
		ae.elemEnc(e, v.Index(i))
	}
	e.WriteByte('}')
}

func newArrayEncoder(t reflect.Type) encoderFunc {
	enc := arrayEncoder{typeEncoder(t.Elem())}
	return enc.encode
}

type ptrEncoder struct {
	elemEnc encoderFunc
}

func (pe *ptrEncoder) encode(e *encodeState, v reflect.Value) {
	if v.IsNil() {
		e.WriteByte(undefined)
		return
	}
	pe.elemEnc(e, v.Elem())
}

func newPtrEncoder(t reflect.Type) encoderFunc {
	enc := ptrEncoder{typeEncoder(t.Elem())}
	return enc.encode

}

type condAddrEncoder struct {
	canAddrEnc, elseEnc encoderFunc
}

func (ce *condAddrEncoder) encode(e *encodeState, v reflect.Value) {
	if v.CanAddr() {
		ce.canAddrEnc(e, v)
	} else {
		ce.elseEnc(e, v)
	}
}

// newCondAddrEncoder returns an encoder that checks whether its value
// CanAddr and delegates to canAddrEnc if so, else to elseEnc.
func newCondAddrEncoder(canAddrEnc, elseEnc encoderFunc) encoderFunc {
	enc := &condAddrEncoder{canAddrEnc: canAddrEnc, elseEnc: elseEnc}
	return enc.encode
}

func timeEncoder(e *encodeState, v reflect.Value) {
	t, ok := v.Interface().(time.Time)
	if !ok {
		e.error(errors.New("can not convert to time.Time"))
	}

	e.WriteByte('D')
	e.WriteByte('/')
	year, month, day := t.Date()
	e.WriteString(strconv.Itoa(year))
	e.WriteByte('/')
	e.WriteString(strconv.Itoa(int(month)))
	e.WriteByte('/')
	e.WriteString(strconv.Itoa(day))
	e.WriteByte(':')
	e.WriteString(strconv.Itoa(t.Hour()))
	e.WriteByte(':')
	e.WriteString(strconv.Itoa(t.Minute()))
	e.WriteByte(':')
	e.WriteString(strconv.Itoa(t.Second()))
}

func isValidTag(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:<=>?@[]^_{|}~ ", c):
			// backslash and quote chars are reserved, but
			// otherwise any punctuation chars are allowed
			// in a tag name
		default:
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
				return false
			}
		}
	}
	return true
}

func typeByIndex(t reflect.Type, index []int) reflect.Type {
	for _, i := range index {
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		t = t.Field(i).Type
	}
	return t
}

type reflectWithString struct {
	v reflect.Value
	s string
}

func (w *reflectWithString) resolve() error {
	if w.v.Kind() == reflect.String {
		w.s = w.v.String()
		return nil
	}
	if tm, ok := w.v.Interface().(encoding.TextMarshaler); ok {
		buf, err := tm.MarshalText()
		w.s = string(buf)
		return err
	}

	switch w.v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		w.s = strconv.FormatInt(w.v.Int(), 10)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		w.s = strconv.FormatUint(w.v.Uint(), 10)
		return nil
	}
	panic("unexpected map key type")
}

// WriteEncode writes encoded v in buffer.
func (e *encodeState) WriteEncode(v interface{}) {
	typeEncoder(reflect.TypeOf(v))(e, reflect.ValueOf(v))
}

// WriteStringValue writes string value framing quotes.
func (e *encodeState) WriteStringValue(s string) {
	e.string(s)
}

// string writes string as value.
func (e *encodeState) string(s string) {
	e.WriteByte('\'')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if safeSet[b] {
				i++
				continue
			}

			if start < i {
				e.WriteString(s[start:i])
			}

			e.WriteByte('\\')
			switch b {
			case '\\', '"', '\'':
				e.WriteByte(b)
			case '\n':
				e.WriteByte('n')
			case '\r':
				e.WriteByte('r')
			case '\t':
				e.WriteByte('t')
			default:
				e.WriteString(`u00`)
				e.WriteByte(hex[b>>4])
				e.WriteByte(hex[b&0xF])
			}
			i++
			start = i
			continue
		}

		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				e.WriteString(s[start:i])
			}

			e.WriteString(`\ufffd`)
			i += size
			start = i
			continue
		}
		i += size
	}

	if start < len(s) {
		e.WriteString(s[start:])
	}

	e.WriteByte('\'')
}

// keep in sync with string above.
func (e *encodeState) stringBytes(s []byte) {
	e.WriteByte('\'')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if safeSet[b] {
				i++
				continue
			}
			if start < i {
				e.Write(s[start:i])
			}
			e.WriteByte('\\')
			switch b {
			case '\\', '"', '\'':
				e.WriteByte(b)
			case '\n':
				e.WriteByte('n')
			case '\r':
				e.WriteByte('r')
			case '\t':
				e.WriteByte('t')
			default:
				e.WriteString(`u00`)
				e.WriteByte(hex[b>>4])
				e.WriteByte(hex[b&0xF])
			}
			i++
			start = i
			continue
		}

		c, size := utf8.DecodeRune(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				e.Write(s[start:i])
			}

			e.WriteString(`\ufffd`)
			i += size
			start = i
			continue
		}
		i += size
	}

	if start < len(s) {
		e.Write(s[start:])

	}
	e.WriteByte('\'')
}

// A field represents a single field found in a struct.
type field struct {
	name      string
	tag       bool
	nameBytes []byte
	equalFold func(s, t []byte) bool // bytes.EqualFold or equivalent
	index     []int
	typ       reflect.Type
	omitEmpty bool
	value     reflect.Value // value in tag
	encoder   encoderFunc
}

// byIndex sorts field by index sequence.
type byIndex []field

func (x byIndex) Len() int { return len(x) }

func (x byIndex) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

func (x byIndex) Less(i, j int) bool {
	for k, xik := range x[i].index {
		if k >= len(x[j].index) {
			return false
		}
		if xik != x[j].index[k] {
			return xik < x[j].index[k]
		}
	}
	return len(x[i].index) < len(x[j].index)
}

// typeFields returns a list of fields that Oscript should recognize for the given type.
// The algorithm is breadth-first search over the set of structs to include - the top struct
// and then any reachable anonymous structs.
func typeFields(t reflect.Type) []field {
	// Anonymous fields to explore at the current level and the next.
	current := []field{}
	next := []field{{typ: t}}

	// count of queued names for current level and the next.
	count := map[reflect.Type]int{}
	nextCount := map[reflect.Type]int{}

	// Types already visited at an earlier level.
	visited := map[reflect.Type]bool{}
	// Fields found.
	var fields []field
	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, map[reflect.Type]int{}
		for _, f := range current {
			if visited[f.typ] {
				continue
			}
			visited[f.typ] = true

			// scan f.typ for fields to include.
			for i := 0; i < f.typ.NumField(); i++ {
				sf := f.typ.Field(i)
				tag := sf.Tag.Get("oscript")
				if tag == "-" {
					continue
				}
				name, opts := parseTag(tag)

				isUnexported := sf.PkgPath != ""
				if sf.Anonymous {
					t := sf.Type
					if t.Kind() == reflect.Ptr {
						t = t.Elem()
					}
					if isUnexported && t.Kind() != reflect.Struct {
						// Ignore embedded fields of unexported non-struct types.
						continue
					}
					// Do not ignore embedded fields of unexported struct types
					// since they may have exported fields.
				} else if isUnexported && !opts.Contains("public") {
					// Ignore unexported non-embedded fields except public in tag
					continue
				}

				if !isValidTag(name) {
					name = ""
				}
				index := make([]int, len(f.index)+1)
				copy(index, f.index)
				index[len(f.index)] = i

				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Ptr {
					// Follow pointer.
					ft = ft.Elem()
				}

				// record found field and index sequence.
				if name != "" || !sf.Anonymous || ft.Kind() != reflect.Struct {
					tagged := name != ""
					if name == "" {
						name = sf.Name
					}

					field := field{
						name:      name,
						tag:       tagged,
						index:     index,
						typ:       ft,
						omitEmpty: opts.Contains("omitempty"),
					}

					if sf.Type == sdoType {
						value := sdoName("'" + name + "'")
						field.name = "_SDOName"
						field.encoder = value.encode
						field.value = reflect.ValueOf(&value).Elem()
					}

					field.nameBytes = []byte(field.name)
					field.equalFold = foldFunc(field.nameBytes)

					fields = append(fields, field)
					if count[f.typ] > 1 {
						// If there were multiple instances, add a second,
						// so that the annihilation code will see a duplicate.
						// It only cares about the distinction between 1 or 2,
						// so don't bother generating any more copies.
						fields = append(fields, fields[len(fields)-1])
					}
					continue
				}
				// Record new anonymous struct to explore in next round.
				nextCount[ft]++
				if nextCount[ft] == 1 {
					next = append(next, field{name: ft.Name(), index: index, typ: ft})
				}
			}
		}
	}

	sort.Slice(fields, func(i, j int) bool {
		x := fields
		// sort field by name, breaking ties with depth, then
		// breaking ties with "name came from oscript tag", then
		// breaking ties with index sequence.
		if x[i].name != x[j].name {
			return x[i].name < x[j].name
		}

		if len(x[i].index) != len(x[j].index) {
			return len(x[i].index) < len(x[j].index)
		}

		if x[i].tag != x[j].tag {
			return x[i].tag
		}
		return byIndex(x).Less(i, j)
	})
	// Delete all fields that are hidden by the Go rules for embedded fields,
	// except that fields with OSCRIPT tags are promoted.
	// The fields are sorted in primary order of name, secondary order
	// of field index length. Loop over names; for each name, delete
	// hidden fields by choosing the one dominant field that survives.
	out := fields[:0]
	for advance, i := 0, 0; i < len(fields); i += advance {
		// One iteration per name.
		// Find the sequence of fields with the name of this first field.
		fi := fields[i]
		name := fi.name

		for advance = 1; i+advance < len(fields); advance++ {
			fj := fields[i+advance]
			if fj.name != name {
				break
			}
		}

		if advance == 1 { // only one field with this name
			out = append(out, fi)
			continue
		}
		dominant, ok := dominantField(fields[i : i+advance])
		if ok {
			out = append(out, dominant)
		}
	}

	fields = out
	sort.Sort(byIndex(fields))

	for i := range fields {
		f := &fields[i]

		if f.encoder != nil {
			continue
		}
		f.encoder = typeEncoder(typeByIndex(t, f.index))
	}
	return fields
}

// dominantField looks through the fields, all of which are known to
// have the same name, to find the single field that dominates the
// others using Go's embedding rules, modified by the presence of
// oscript tags. If there are multiple top-level fields, the boolean
// will be false: This condition is an error in Go and we skip all
// the fields.
func dominantField(fields []field) (field, bool) {
	// the fields are sorted in increasing index-length order. The winner
	// must therefore be one with the shortest index length. Drop all
	// longer entries, which is easy: just truncate the slice.
	length := len(fields[0].index)
	tagged := -1 // Index of first tagged field.
	for i, f := range fields {
		if len(f.index) > length {
			fields = fields[:i]
			break
		}

		if f.tag {
			if tagged >= 0 {
				// Multiple tagged fields at the same level: conflict.
				// Return no field.
				return field{}, false
			}
			tagged = i
		}
	}

	if tagged >= 0 {
		return fields[tagged], true
	}

	// All remaining fields have the same length. If there's more than one,
	// we have a conflict (two fields named "X" at the same level) and we
	// return no field.
	if len(fields) > 1 {
		return field{}, false
	}
	return fields[0], true
}

var fieldCache sync.Map // map[reflect.Type][]field

// cachedTypeFields is like typeFields but uses a cache to avoid repeated work.
func cachedTypeFields(t reflect.Type) []field {
	if f, ok := fieldCache.Load(t); ok {
		return f.([]field)
	}
	f, _ := fieldCache.LoadOrStore(t, typeFields(t))
	return f.([]field)
}

package oscript

// Represents oscript data structure using native Go types: booleans, floats,
// strings, arrays, and maps.
import (
	"bytes"
	"encoding"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Unmarshal parses the oscript-encoded data and stores the result
// in the value pointed to by v. If v is nil or not a pointer,
// Unmarshal returns an InvalidUnmarshalError.
//
// Unmarshal uses the inverse of the encodings that
// Marshal uses, allocating maps, slices, and pointers as necessary,
// with the following additional rules:
//
// To unmarshal oscript into a pointer, Unmarshal first handles the case of
// the oscript being the oscript literal undefined(?). In that case, Unmarshal sets
// the pointer to nil. Otherwise, Unmarshal unmarshals the oscript into
// the value pointed at by the pointer. If the pointer is nil, Unmarshal
// allocates a new value for it to point to.
//
// To unmarshal oscript into a value implementing the Unmarshaler interface,
// Unmarshal calls that value's UnmarshalOscript method, including
// when the input is a oscript undefined.
//
// To unmarshal oscript into a struct, Unmarshal matches incoming object
// keys to the keys used by Marshal (either the struct field name or its tag),
// preferring an exact match but also accepting a case-insensitive match. By
// default, object keys which don't have a corresponding struct field are
// ignored (see Decoder.DisallowUnknownFields for an alternative).
//
// To unmarshal oscript into an interface value,
// Unmarshal stores one of these in the interface value:
//
//	bool, for oscript booleans
//	float64, for oscript reals
//	string, for oscript strings
//	int, for oscript integers
//  time.Time, for oscript dates
//  oscript.Error, for oscript errors
//	[]interface{}, for oscript arrays
//	map[string]interface{} struct, for oscript assocs
//	nil for oscript undefined
//
// To unmarshal a Oscript array into a slice, Unmarshal resets the slice length
// to zero and then appends each element to the slice.
// As a special case, to unmarshal an empty Oscript array into a slice,
// Unmarshal replaces the slice with a new empty slice.
//
// To unmarshal a oscript array into a Go array, Unmarshal decodes
// Oscript array elements into corresponding Go array elements.
// If the Go array is smaller than the Oscript array,
// the additional Oscript array elements are discarded.
// If the Oscript array is smaller than the Go array,
// the additional Go array elements are set to zero values.
//
// To unmarshal a oscript object into a map, Unmarshal first establishes a map to
// use. If the map is nil, Unmarshal allocates a new map. Otherwise Unmarshal
// reuses the existing map, keeping existing entries. Unmarshal then stores
// key-value pairs from the Oscript object into the map. The map's key type must
// either be a string, an integer, or implement encoding.TextUnmarshaler.
//
// To Unmarshal a oscript service data object into a Go struct has
// SDOName. Namespace and name of the struct must equal oscript service data object.

// If a oscript value is not appropriate for a given target type,
// or if a oscript number overflows the target type, Unmarshal
// skips that field and completes the unmarshaling as best it can.
// If no more serious errors are encountered, Unmarshal returns
// an UnmarshalTypeError describing the earliest such error. In any
// case, it's not guaranteed that all the remaining fields following
// the problematic one will be unmarshaled into the target object.
//

// The oscript undefined value unmarshals into an interface, map, pointer, or slice
// by setting that Go value to nil. Because undefined is often used in oscript to mean
// `not present' unmarshaling a Oscript undefined into any other Go type has no effect
// on the value and produces no error.
//
// When unmarshaling quoted strings, invalid UTF-8 or
// invalid UTF-16 surrogate pairs are not treated as an error.
// Instead, they are replaced by the Unicode replacement
// character U+FFFD.
//
func Unmarshal(data []byte, v interface{}) error {
	// Check for well-formedness.
	// Avoids filling want half a data structure
	// before discovering a oscript syntax error.
	var d decodeState
	err := checkValid(data, &d.scan)
	if err != nil {
		return err
	}
	d.init(data)
	return d.unmarshal(v)
}

// Unmarshaler is the interface implemented by types
// that can unmarshal a Oscript description of themselves.
// The input can be assumed to be a valid encoding of
// a Oscript value. UnmarshalOscript must copy the Oscript data
// if it wishes to retain the data after returning.
//
// By convention, to approximate the behavior of Unmarshal itself,
// Unmarshalers implement UnmarshalOscript([]byte("?")) as a no-op.
type Unmarshaler interface {
	UnmarshalOscript([]byte) error
}

// An UnmarshalTypeError describes a Oscript value that was
// not appropriate for a value of a specific Go type.
type UnmarshalTypeError struct {
	Value  string       // description of Oscript value - "bool", "array", "number -5"
	Type   reflect.Type // type of Go value it could not be assigned to
	Offset int64        // error occurred after reading Offset bytes
	Struct string       // name of the struct type containing the field
	Field  string       // name of the field holding the Go value
}

func (e *UnmarshalTypeError) Error() string {
	if e.Struct != "" || e.Field != "" {
		return "oscript: cannot unmarshal " + e.Value + " into Go struct field " + e.Struct + "." + e.Field + " of type " + e.Type.String()
	}
	return "oscript: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String()
}

// An InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
// (The argument to Unmarshal must be a non-nil pointer.)
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "oscript: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Ptr {
		return "oscript: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "oscript: Unmarshal(nil " + e.Type.String() + ")"
}

func (d *decodeState) unmarshal(v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	d.scan.reset()
	d.scanWhile(scanSkipSpace)
	// We decode rv not rv.Elem because the Unmarshaler interface
	// test must be applied at the top level of the value.
	err := d.value(rv)
	if err != nil {
		return err
	}
	return d.savedError
}

// decodeState represents the state while decoding a Oscript value.
type decodeState struct {
	data         []byte
	off          int // next read offset in data
	opcode       int // last read result
	scan         scanner
	errorContext struct { // provides context for type errors
		Struct string
		Field  string
	}
	savedError            error
	disallowUnknownFields bool
}

// readIndex returns the position of the last byte read.
func (d *decodeState) readIndex() int {
	return d.off - 1
}

// errPhase is used for errors that should not happen unless
// there is a bug in the Oscript decoder or something is editing
// the data slice while the decoder executes.
var errPhase = errors.New("oscript decoder want of sync - data changing underfoot")

func (d *decodeState) init(data []byte) *decodeState {
	d.data = data
	d.off = 0
	d.savedError = nil
	d.errorContext.Struct = ""
	d.errorContext.Field = ""
	return d
}

// saveError saves the first err it is called with,
// for reporting at the end of the unmarshal.
func (d *decodeState) saveError(err error) {
	if d.savedError == nil {
		d.savedError = d.addErrorContext(err)
	}
}

// addErrorContext returns a new error enhanced with information from d.errorContext
func (d *decodeState) addErrorContext(err error) error {
	if d.errorContext.Struct != "" || d.errorContext.Field != "" {
		switch err := err.(type) {
		case *UnmarshalTypeError:
			err.Struct = d.errorContext.Struct
			err.Field = d.errorContext.Field
			return err
		}
	}
	return err
}

// skip scans to the end of what was started.
func (d *decodeState) skip() {
	s, data, i := &d.scan, d.data, d.off
	depth := len(s.parseState)
	for {
		op := s.step(s, data[i])
		i++
		if len(s.parseState) < depth {
			d.off = i
			d.opcode = op
			return
		}
	}
}

// scanNext processes the byte at d.data[d.off].
func (d *decodeState) scanNext() {
	s, data, i := &d.scan, d.data, d.off
	if i < len(data) {
		d.opcode = s.step(s, data[i])
		d.off = i + 1
	} else {
		d.opcode = s.eof()
		d.off = len(data) + 1 // mark processed EOF with len+1
	}
}

// scanWhile processes bytes in d.data[d.off:] until it
// receives a scan code not equal to op.
func (d *decodeState) scanWhile(op int) {
	s, data, i := &d.scan, d.data, d.off
	for i < len(d.data) {
		newOp := s.step(s, data[i])
		i++
		if newOp != op {
			d.opcode = newOp
			d.off = i
			return
		}
	}

	d.off = len(d.data) + 1 // mark processed EOF with len+1
	d.opcode = d.scan.eof()
}

// value consumes a oscript value from d.data[d.off:], decoding into the v and reads
// the following byte ahead. If v is invalid, the value is discarded.
// The first byte of the value has been read already.
func (d *decodeState) value(v reflect.Value) error {
	switch d.opcode {
	default:
		return errPhase

	case scanBeginArray:
		if v.IsValid() {
			if err := d.array(v); err != nil {
				return err
			}
		} else {
			d.skip()
		}
		d.scanNext()

	case scanBeginObject:
		d.scanWhile(scanContinue)
		if v.IsValid() {
			if err := d.object(v); err != nil {
				return err
			}
		} else {
			d.skip()
		}
		d.scanNext()

	case scanBeginLiteral:
		// all bytes inside literal return scanContinue op code
		start := d.readIndex()
		d.scanWhile(scanContinue)

		if v.IsValid() {
			if err := d.literalStore(d.data[start:d.readIndex()], v); err != nil {
				return err
			}
		}
	}
	return nil
}

// indirect walks down v allocating pointers as needed, until it gets to a non-pointer.
// if it encounters an Unmarshaler, indirect stops and returns that.
// if decodingNull is true, indirect stops at the last pointer so it can be set to nil.
func (d *decodeState) indirect(v reflect.Value, decodingNull bool) (Unmarshaler, reflect.Value) {
	// Issue #24153 indicates that it is generally not a guaranteed property
	// that you may round-trip a reflect.Value by calling Value.Addr().Elem()
	// and expect the value to still be settable for values derived from
	// unexported embedded struct fields.
	//
	// The logic below effectively does this when it first addresses the value
	// (to satisfy possible pointer methods) and continues to dereference
	// subsequent pointers as necessary.
	//
	// After the first round-trip, we set v back to the original value to
	// preserve the original RW flags contained in reflect.Value.
	v0 := v
	haveAddr := false

	// If v is a named type and is addressable,
	// start with its address, so that if the type has pointer methods,
	// we find them.
	if v.Kind() != reflect.Ptr && v.Type().Name() != "" && v.CanAddr() {
		haveAddr = true
		v = v.Addr()
	}
	for {
		// load value from interface, but only if the result will be usefully addressable
		if v.Kind() == reflect.Interface && !v.IsNil() {
			e := v.Elem()
			if e.Kind() == reflect.Ptr && !e.IsNil() && (!decodingNull || e.Elem().Kind() == reflect.Ptr) {
				haveAddr = false
				v = e
				continue
			}
		}

		if v.Kind() != reflect.Ptr {
			break
		}

		if v.Elem().Kind() != reflect.Ptr && decodingNull && v.CanSet() {
			break
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		if v.Type().NumMethod() > 0 && v.CanInterface() {
			if u, ok := v.Interface().(Unmarshaler); ok {
				return u, reflect.Value{}
			}
		}

		if haveAddr {
			v = v0 // restore original value after round-trip Value.Addr().Elem()
			haveAddr = false
		} else {
			v = v.Elem()
		}
	}
	return nil, v
}

// array consumes an array from d.data[d.off-1:], decoding into the value v.
// the first byte of the array ('{') has been read already.
func (d *decodeState) array(v reflect.Value) error {
	// Check for unmarshaler.
	u, pv := d.indirect(v, false)
	if u != nil {
		start := d.readIndex()
		d.skip()
		return u.UnmarshalOscript(d.data[start:d.off])
	}
	v = pv

	// check type of target
	switch v.Kind() {
	case reflect.Interface:
		if v.NumMethod() == 0 {
			//decoding into nil interface, switch to non-reflect code
			ai, err := d.arrayInterface()
			if err != nil {
				return err
			}
			v.Set(reflect.ValueOf(ai))
			return nil
		}
		// otherwise it's invalid
		fallthrough

	default:
		d.saveError(&UnmarshalTypeError{Value: "array", Type: v.Type(), Offset: int64(d.off)})
		d.skip()
		return nil

	case reflect.Array:
	case reflect.Slice:
		break
	}

	i := 0
	for {
		// Look ahead for } - can only happen on first iteration.
		d.scanWhile(scanSkipSpace)
		if d.opcode == scanEndArray {
			break
		}

		// Get element of array, growing if necessary.
		if v.Kind() == reflect.Slice {
			// Grow slice if necessary
			if i >= v.Cap() {
				newcap := v.Cap() + v.Cap()/2
				if newcap < 4 {
					newcap = 4
				}
				newv := reflect.MakeSlice(v.Type(), v.Len(), newcap)
				reflect.Copy(newv, v)
				v.Set(newv)
			}

			if i >= v.Len() {
				v.SetLen(i + 1)
			}
		}

		if i < v.Len() {
			// Decode into element.
			if err := d.value(v.Index(i)); err != nil {
				return err
			}
		} else {
			// Ran want of fixed array: skip.
			if err := d.value(reflect.Value{}); err != nil {
				return err
			}
		}
		i++

		// Next token must be , or }.
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndArray {
			break
		}
		if d.opcode != scanArrayValue {
			return errPhase
		}

	}

	if i < v.Len() {
		if v.Kind() == reflect.Array {
			// Array. Zero the rest.
			z := reflect.Zero(v.Type().Elem())
			for ; i < v.Len(); i++ {
				v.Index(i).Set(z)
			}
		} else {
			v.SetLen(i)
		}
	}

	if i == 0 && v.Kind() == reflect.Slice {
		v.Set(reflect.MakeSlice(v.Type(), 0, 0))
	}
	return nil
}

var (
	textUnmarshalerType = reflect.TypeOf(new(encoding.TextUnmarshaler)).Elem()
	nilValue            = reflect.ValueOf(nil)
)

// object consumes an object from d.data[d.off-1:], decoding into the value v.
// the first byte 'A' of the object has been read already.
func (d *decodeState) object(v reflect.Value) error {
	// Check for unmarshaler.
	u, pv := d.indirect(v, false)
	if u != nil {
		start := d.readIndex()
		d.skip()
		return u.UnmarshalOscript(d.data[start:d.off])
	}
	v = pv

	// Decoding into nil interface?  Switch to non-reflect code.
	if v.Kind() == reflect.Interface && v.NumMethod() == 0 {
		oi, err := d.objectInterface()
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(oi))
		return nil
	}

	// Check type of target:
	//   struct or
	//   map[T1]T2 where T1 is string, an integer type, or an encoding.TextUnmarshaler
	switch v.Kind() {
	case reflect.Map:
		// Map key must either have string kind, have an integer kind,
		// or be an encoding.TextUnmarshaler.
		t := v.Type()
		switch t.Key().Kind() {
		case reflect.String,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		default:
			if !reflect.PtrTo(t.Key()).Implements(textUnmarshalerType) {
				d.saveError(&UnmarshalTypeError{Value: "object", Type: v.Type(), Offset: int64(d.off + 1)}) // + 1, because we has not read `,`
				d.skip()
				return nil
			}
		}
		if v.IsNil() {
			v.Set(reflect.MakeMap(t))
		}
	case reflect.Struct:
		// ok
	default:
		d.saveError(&UnmarshalTypeError{Value: "object", Type: v.Type(), Offset: int64(d.off + 1)})
		d.skip() // skip over A<1,? > in input
		return nil
	}

	var mapElem reflect.Value
	originalErrorContext := d.errorContext

	// read `,` value or closing >
	d.scanNext()
	if d.opcode == scanEndObject {
		return nil
	}

	for {
		// Read opening ' of string key or closing >.
		d.scanWhile(scanSkipSpace)
		if d.opcode == scanEndObject {
			// closing > - can only happen on end iteration.
			break
		}

		if d.opcode != scanBeginLiteral {
			return errPhase
		}

		// Read key.
		start := d.readIndex()
		d.scanWhile(scanContinue)
		item := d.data[start:d.readIndex()]
		key, ok := unquoteBytes(item)
		if !ok {
			return errPhase
		}

		// Figure want field corresponding to key.
		var subv reflect.Value

		if v.Kind() == reflect.Map {
			elemType := v.Type().Elem()
			if !mapElem.IsValid() {
				mapElem = reflect.New(elemType).Elem()
			} else {
				mapElem.Set(reflect.Zero(elemType))
			}
			subv = mapElem
		} else {
			var f *field
			fields := cachedTypeFields(v.Type())
			for i := range fields {
				ff := &fields[i]
				if bytes.Equal(ff.nameBytes, key) {
					f = ff
					break
				}
				if f == nil && ff.equalFold(ff.nameBytes, key) {
					f = ff
				}
			}

			if f != nil {
				if f.value != nilValue {
					subv = f.value
				} else {
					subv = v
					for _, i := range f.index {
						if subv.Kind() == reflect.Ptr {
							if subv.IsNil() {
								// If a struct embeds a pointer to an unexported type,
								// it is not possible to set a newly allocated value
								// since the field is unexported.
								if !subv.CanSet() {
									d.saveError(fmt.Errorf("oscript: cannot set embedded pointer to unexported struct: %v", subv.Type().Elem()))
									// Invalidate subv to ensure d.value(subv) skips over
									// the Oscript value without assigning it to subv.
									subv = reflect.Value{}
									break
								}
								subv.Set(reflect.New(subv.Type().Elem()))
							}
							subv = subv.Elem()
						}
						subv = subv.Field(i)
					}
				}
				d.errorContext.Field = f.name
				d.errorContext.Struct = v.Type().Name()
			} else if d.disallowUnknownFields {
				d.saveError(fmt.Errorf("oscript: unknown field %q", key))
			}
		}

		// Read = before value.
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode != scanObjectKey {
			return errPhase
		}
		d.scanWhile(scanSkipSpace)

		if err := d.value(subv); err != nil {
			return err
		}

		// Write value back to map;
		// if using struct without, subv points into struct already.
		if v.Kind() == reflect.Map {
			kt := v.Type().Key()
			var kv reflect.Value
			switch {
			case kt.Kind() == reflect.String:
				kv = reflect.ValueOf(key).Convert(kt)
			case reflect.PtrTo(kt).Implements(textUnmarshalerType):
				kv = reflect.New(v.Type().Key())
				if err := d.literalTextUnmarshaler(item, kv); err != nil {
					return err
				}
				kv = kv.Elem()

			default:
				switch kt.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					s := string(key)
					n, err := strconv.ParseInt(s, 10, 64)
					if err != nil || reflect.Zero(kt).OverflowInt(n) {
						d.saveError(&UnmarshalTypeError{Value: "number " + s, Type: kt, Offset: int64(start + 1)})
						return nil
					}
					kv = reflect.ValueOf(n).Convert(kt)
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
					s := string(key)
					n, err := strconv.ParseUint(s, 10, 64)
					if err != nil || reflect.Zero(kt).OverflowUint(n) {
						d.saveError(&UnmarshalTypeError{Value: "number " + s, Type: kt, Offset: int64(start + 1)})
						return nil
					}
					kv = reflect.ValueOf(n).Convert(kt)
				default:
					panic("oscript: unexpected key type") // should never occur
				}
			}
			v.SetMapIndex(kv, subv)
		}

		// Next token must be , or >.
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndObject {
			break
		}
		if d.opcode != scanObjectValue {
			return errPhase
		}

		d.errorContext = originalErrorContext
	}

	return nil
}

// convertNumber converts the number literal s to a int64.
func (d *decodeState) convertNumber(s string) (interface{}, error) {
	f, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, &UnmarshalTypeError{Value: "number " + s, Type: reflect.TypeOf(0.0), Offset: int64(d.off)}
	}
	return f, nil
}

// convertFloat converts the number literal s to a float64.
func (d *decodeState) convertFloat(s string) (interface{}, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, &UnmarshalTypeError{Value: "number " + s, Type: reflect.TypeOf(0.0), Offset: int64(d.off)}
	}
	return f, nil
}

var regTime = regexp.MustCompile(`D/(\d+)/(\d{1,2})/(\d{1,2}):(\d{1,2}):(\d{1,2}):(\d{1,2})`)

// convertTime converts the date s to a time.Time.
func convertTime(s string) (time.Time, error) {
	d := regTime.FindStringSubmatch(s)
	if d == nil || len(d) != 7 {
		return time.Time{}, fmt.Errorf("can not parse %s", s)
	}

	year, err := strconv.Atoi(d[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("can not parse %s", s)
	}

	month, err := strconv.Atoi(d[2])
	if err != nil {
		return time.Time{}, fmt.Errorf("can not parse %s", s)
	}

	day, err := strconv.Atoi(d[3])
	if err != nil {
		return time.Time{}, fmt.Errorf("can not parse %s", s)
	}

	hour, err := strconv.Atoi(d[4])
	if err != nil {
		return time.Time{}, fmt.Errorf("can not parse %s", s)
	}

	minute, err := strconv.Atoi(d[5])
	if err != nil {
		return time.Time{}, fmt.Errorf("can not parse %s", s)
	}

	sec, err := strconv.Atoi(d[6])
	if err != nil {
		return time.Time{}, fmt.Errorf("can not parse %s", s)
	}

	return time.Date(year, time.Month(month), day, hour, minute, sec, 0, time.UTC), nil
}

// literalStore decodes a literal stored in item into v.
func (d *decodeState) literalStore(item []byte, v reflect.Value) error {
	// Check for unmarshaler.
	if len(item) == 0 {
		d.saveError(fmt.Errorf("oscript: empty string given, trying to unmarshal %q into %v", item, v.Type()))
		return nil
	}

	isUndefined := item[0] == '?' // undefined
	u, pv := d.indirect(v, isUndefined)
	if u != nil {
		return u.UnmarshalOscript(item)
	}

	v = pv
	switch c := item[0]; c {
	case '?': // undefined value
		switch v.Kind() {
		case reflect.Interface, reflect.Ptr, reflect.Map, reflect.Slice:
			v.Set(reflect.Zero(v.Type()))
			// otherwise, ignore undefined for primitives/string
		}

	case 'D': // time value
		switch v.Kind() {
		default:
			d.saveError(&UnmarshalTypeError{Value: "time", Type: v.Type(), Offset: int64(d.readIndex())})

		case reflect.Struct:
			if v.Type() == timeType {
				t, err := convertTime(string(item))
				if err != nil {
					d.saveError(err)
					break
				}
				v.Set(reflect.ValueOf(t))
			} else {
				d.saveError(&UnmarshalTypeError{Value: "time", Type: v.Type(), Offset: int64(d.readIndex())})
			}

		case reflect.Interface:
			if v.NumMethod() == 0 {
				t, err := convertTime(string(item))
				if err != nil {
					d.saveError(err)
					break
				}
				v.Set(reflect.ValueOf(t))
			} else {
				d.saveError(&UnmarshalTypeError{Value: "time", Type: v.Type(), Offset: int64(d.readIndex())})
			}
		}

	case 't', 'f': // true, false
		value := item[0] == 't'
		switch v.Kind() {
		default:
			d.saveError(&UnmarshalTypeError{Value: "bool", Type: v.Type(), Offset: int64(d.readIndex())})

		case reflect.Bool:
			v.SetBool(value)

		case reflect.Interface:
			if v.NumMethod() == 0 {
				v.Set(reflect.ValueOf(value))
			} else {
				d.saveError(&UnmarshalTypeError{Value: "bool", Type: v.Type(), Offset: int64(d.readIndex())})
			}
		}

	case '\'': // string
		s, ok := unquoteBytes(item)
		if !ok {
			return errPhase
		}

		switch v.Kind() {
		default:
			d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})

		case reflect.Slice:
			if v.Type().Elem().Kind() != reflect.Uint8 {
				d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}

			b := make([]byte, base64.StdEncoding.DecodedLen(len(s)))
			n, err := base64.StdEncoding.Decode(b, s)
			if err != nil {
				d.saveError(err)
				break
			}
			v.SetBytes(b[:n])

		case reflect.String:
			v.SetString(string(s))

		case reflect.Interface:
			if v.NumMethod() == 0 {
				v.Set(reflect.ValueOf(string(s)))
			} else {
				d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})
			}
		}
	case 'G': // float
		if c = item[1]; c != '-' && (c < '0' || c > '9') {
			return errPhase
		}
		s := string(item[1:])
		switch v.Kind() {
		default:
			d.saveError(&UnmarshalTypeError{Value: "float", Type: v.Type(), Offset: int64(d.readIndex())})

		case reflect.Interface:
			n, err := d.convertFloat(s)
			if err != nil {
				d.saveError(err)
				break
			}

			if v.NumMethod() != 0 {
				d.saveError(&UnmarshalTypeError{Value: "float", Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.Set(reflect.ValueOf(n))

		case reflect.Float32, reflect.Float64:
			n, err := strconv.ParseFloat(s, v.Type().Bits())
			if err != nil || v.OverflowFloat(n) {
				d.saveError(&UnmarshalTypeError{Value: "float" + s, Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.SetFloat(n)
		}

	default: // int
		if c == 'L' {
			c = item[1]
			item = item[1:]
		}
		if c != '-' && (c < '0' || c > '9') {
			return errPhase
		}

		s := string(item)
		switch v.Kind() {
		default:
			d.saveError(&UnmarshalTypeError{Value: "int", Type: v.Type(), Offset: int64(d.readIndex())})

		case reflect.Interface:
			n, err := d.convertNumber(s)
			if err != nil {
				d.saveError(err)
				break
			}

			if v.NumMethod() != 0 {
				d.saveError(&UnmarshalTypeError{Value: "int", Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.Set(reflect.ValueOf(n))

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil || v.OverflowInt(n) {
				d.saveError(&UnmarshalTypeError{Value: "int " + s, Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.SetInt(n)

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			n, err := strconv.ParseUint(s, 10, 64)
			if err != nil || v.OverflowUint(n) {
				d.saveError(&UnmarshalTypeError{Value: "int " + s, Type: v.Type(), Offset: int64(d.readIndex())})
				break
			}
			v.SetUint(n)
		}
	}

	return nil
}

// literalTextUnmarshaler decodes a literal stored in item into v,
// which implemented encoding.TextUnmarshaler.
//
// From rules: The map's key type must either be a string, an integer,
// or implement encoding.TextUnmarshaler.
func (d *decodeState) literalTextUnmarshaler(item []byte, v reflect.Value) error {
	// Error for unmarshaler.
	if len(item) == 0 {
		d.saveError(fmt.Errorf("oscript: empty string given, trying to unmarshal %q into %v", item, v.Type()))
		return nil
	}

	u := v.Interface().(encoding.TextUnmarshaler)
	s, ok := unquoteBytes(item)
	if !ok {
		return errPhase
	}

	return u.UnmarshalText(s)
}

// The xxxInterface routines build up a value to be stored
// in an empty interface. They are not strictly necessary,
// but they avoid the weight of reflection in this common case.

// valueInterface is like value but returns interface{}
func (d *decodeState) valueInterface() (val interface{}, err error) {
	switch d.opcode {
	default:
		err = errPhase
	case scanBeginArray:
		val, err = d.arrayInterface()
		d.scanNext()
	case scanBeginObject:
		d.scanWhile(scanContinue)
		val, err = d.objectInterface()
		d.scanNext()
	case scanBeginLiteral:
		val, err = d.literalInterface()
	}
	return
}

// arrayInterface is like array but returns []interface{}.
func (d *decodeState) arrayInterface() ([]interface{}, error) {
	var v = make([]interface{}, 0)
	for {
		// Look ahead for } - can only happen on first iteration.
		d.scanWhile(scanSkipSpace)
		if d.opcode == scanEndArray {
			break
		}

		vi, err := d.valueInterface()
		if err != nil {
			return nil, err
		}
		v = append(v, vi)

		// Next token must be , or }.
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndArray {
			break
		}
		if d.opcode != scanArrayValue {
			return nil, errPhase
		}
	}
	return v, nil
}

// objectInterface is like object but returns map[string]interface{}.
func (d *decodeState) objectInterface() (map[string]interface{}, error) {
	m := make(map[string]interface{})
	// read `,` value or closing >
	d.scanNext()
	if d.opcode == scanEndObject {
		return m, nil
	}

	for {
		// Read opening ' of string key or closing >
		d.scanWhile(scanSkipSpace)
		if d.opcode == scanEndObject {
			// closing > - can only happen on end iteration.
			break
		}

		if d.opcode != scanBeginLiteral {
			return nil, errPhase
		}

		// Read string key.
		start := d.readIndex()
		d.scanWhile(scanContinue)
		item := d.data[start:d.readIndex()]

		key, ok := unquote(item)
		if !ok {
			return nil, errPhase
		}

		// Read = before value.
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode != scanObjectKey {
			return nil, errPhase
		}
		d.scanWhile(scanSkipSpace)

		// Read value.
		vi, err := d.valueInterface()
		if err != nil {
			return nil, err
		}
		m[key] = vi

		// Next token must be , or >.
		if d.opcode == scanSkipSpace {
			d.scanWhile(scanSkipSpace)
		}
		if d.opcode == scanEndObject {
			break
		}
		if d.opcode != scanObjectValue {
			return nil, errPhase
		}
	}
	return m, nil
}

// literalInterface consumes and returns a literal from d.data[d.off-1:] and
// it reads the following byte ahead. The first byte of the literal has been
// read already (that's how the caller knows it's a literal).
func (d *decodeState) literalInterface() (interface{}, error) {
	// All bytes inside literal return scanContinue op code.
	start := d.readIndex()
	d.scanWhile(scanContinue)

	item := d.data[start:d.readIndex()]
	switch c := item[0]; c {
	case '?': // undefined
		return nil, nil

	case 'D': // time value
		t, err := convertTime(string(item))
		if err != nil {
			return nil, err
		}
		return t, nil

	case 't', 'f': // true, false
		return c == 't', nil

	case '\'': // string
		s, ok := unquote(item)
		if !ok {
			return nil, errPhase
		}
		return s, nil

	case 'G': // float
		c = item[1]
		if c != '-' && (c < '0' || c > '9') {
			return nil, errPhase
		}
		n, err := d.convertFloat(string(item[1:]))
		if err != nil {
			return nil, err
		}
		return n, nil
	default: // int
		if c == 'L' {
			c = item[1]
			item = item[1:]
		}
		if c != '-' && (c < '0' || c > '9') {
			return nil, errPhase
		}

		n, err := d.convertNumber(string(item))
		if err != nil {
			return nil, err
		}
		return n, nil
	}
}

// getu4 decodes \uXXXX from the beginning of s, returning the hex value,
// or it returns -1.
func getu4(s []byte) rune {
	if len(s) < 6 || s[0] != '\\' || s[1] != 'u' {
		return -1
	}
	var r rune
	for _, c := range s[2:6] {
		switch {
		case '0' <= c && c <= '9':
			c = c - '0'
		case 'a' <= c && c <= 'f':
			c = c - 'a' + 10
		case 'A' <= c && c <= 'F':
			c = c - 'A' + 10
		default:
			return -1
		}
		r = r*16 + rune(c)
	}
	return r
}

// unquote converts a quoted oscript string literal s into an actual string t.
func unquote(s []byte) (t string, ok bool) {
	s, ok = unquoteBytes(s)
	t = string(s)
	return
}

// bytes may be encoded any rule that is why no checked on valid utf8 and try any way to convert to valid utf8.
func unquoteBytes(s []byte) (t []byte, ok bool) {
	if len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
		return
	}
	s = s[1 : len(s)-1]

	// if there are none, then no unquoting is needed, so return a slice of the original bytes
	r := 0
	for r < len(s) {
		c := s[r]

		if c == '\\' || c == '\'' {
			break
		}

		if c < utf8.RuneSelf {
			r++
			continue
		}

		rr, size := utf8.DecodeRune(s[r:])
		if rr == utf8.RuneError && size == 1 {
			break
		}
		r += size
	}

	if r == len(s) {
		return s, true
	}

	b := make([]byte, len(s)+2*utf8.UTFMax)
	w := copy(b, s[0:r])
	for r < len(s) {
		// can only happen if s is full of
		// malformed UTF-8 and we're replacing each
		// byte with RuneError.
		if w >= len(b)-2*utf8.UTFMax {
			nb := make([]byte, (len(b)+utf8.UTFMax)*2)
			copy(nb, b[0:w])
			b = nb
		}

		switch c := s[r]; {
		case c == '\\':
			r++
			if r >= len(s) {
				return
			}

			switch s[r] {
			default:
				return

			case '"', '\\', '/', '\'':
				b[w] = s[r]
				r++
				w++

			case 'b':
				b[w] = '\b'
				r++
				w++

			case 'f':
				b[w] = '\f'
				r++
				w++

			case 'n':
				b[w] = '\n'
				r++
				w++

			case 'r':
				b[w] = '\r'
				r++
				w++

			case 't':
				b[w] = '\t'
				r++
				w++
			case 'u':
				r--
				rr := getu4(s[r:])
				if rr < 0 {
					return
				}
				r += 6
				if utf16.IsSurrogate(rr) {
					rr1 := getu4(s[r:])
					if dec := utf16.DecodeRune(rr, rr1); dec != unicode.ReplacementChar {
						// a valid pair; consume.
						r += 6
						w += utf8.EncodeRune(b[w:], dec)
						break
					}

					// invalid surrogate; fall back to replacement rune.
					rr = unicode.ReplacementChar
				}
				w += utf8.EncodeRune(b[w:], rr)
			}

			// quote, control characters are invalid
		case c == '\'':
			return

			// invalid utf8, allow
		case c < ' ':
			b[w] = c
			r++
			w++

			// ascii
		case c < utf8.RuneSelf:
			b[w] = c
			r++
			w++

			// coerce to well-formed utf8
		default:
			rr, size := utf8.DecodeRune(s[r:])
			r += size
			w += utf8.EncodeRune(b[w:], rr)
		}
	}

	return b[0:w], true
}

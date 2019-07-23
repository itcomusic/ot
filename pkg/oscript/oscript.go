package oscript

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

// sdoName replaces encoderFunc of the SDOName when using in struct SDOName `oscript:"namespace.name"` and
// writes value which in tag.
type sdoName []byte

func (sn sdoName) UnmarshalOscript(data []byte) error {
	if !bytes.Equal(sn, data) {
		if len(data) >= 2 || data[0] != '\'' || data[len(data)-1] != '\'' {
			data = data[1 : len(data)-1]
		}
		return fmt.Errorf("oscript: unknown SDOName \"%s\"", data)
	}

	return nil
}

// encode implements oscript.encoder.
func (sn sdoName) encode(e *encodeState, _ reflect.Value) {
	e.Write(sn)
}

type SDOName struct{}

// Error is a error string.
type Error int

// MarshalOscript implements the oscript.Marshaler interface.
func (e Error) MarshalOscript() ([]byte, error) {
	return []byte("E" + strconv.Itoa(int(e))), nil
}

// UnmarshalOscript implements the oscript.Unmarshaler interface.
func (e *Error) UnmarshalOscript(data []byte) error {
	if string(data) == "?" {
		return nil
	}

	i, err := strconv.Atoi(string(data[1:]))
	if err != nil {
		return errors.New("oscript: error type, " + err.Error())
	}

	*e = Error(i)
	return nil
}

// Error returns error message.
func (e Error) Error() string {
	return strconv.Itoa(int(e))
}

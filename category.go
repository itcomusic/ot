package ot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/itcomusic/ot/pkg/oscript"
)

var (
	errCategory     = errors.New("not found category")
	errNilTypeValue = errors.New("value is nil")
	errTypeValue    = errors.New("TypeValue is invalid")
)

// Value is the attributes of the category.
type Value struct {
	Description string        `oscript:"Description"`
	Key         string        `oscript:"Key"`
	Value       []interface{} `oscript:"Values"`

	Type TypeValue `oscript:"_SDOName"`
}

// Category of the node.
type Category struct {
	DisplayName string  `oscript:"DisplayName"`
	Key         string  `oscript:"Key"`
	Type        string  `oscript:"Type"`
	Data        []Value `oscript:"Values"`

	sdoName oscript.SDOName `oscript:"DocMan.AttributeGroup,public"`
}

// Copy copies category with values.
func (c *Category) Copy() *Category {
	var values []Value
	if c.Data != nil {
		values = make([]Value, len(c.Data))
		copy(values, c.Data)
	}

	return &Category{
		DisplayName: c.DisplayName,
		Key:         c.Key,
		Type:        c.Type,
		Data:        values,
	}
}

// Upgrade upgrades category.
func (c *Category) Upgrade(new Category) error {
	if c == nil {
		return errCategory
	}

	newID, newVer := new.IDVersion()
	oldID, oldVer := c.IDVersion()

	if oldID != newID || oldVer >= newVer {
		return nil
	}

	// copy values
	values := make([]Value, len(new.Data))
	copy(values, new.Data)
	new.Data = values

	for _, o := range c.Data {
		for i, n := range new.Data {
			if o.Description == n.Description {
				// TODO: converts auto int, time, bool -> string
				if o.Type != n.Type {
					return fmt.Errorf("invalid type attribute \"%s\" \"%s\"", n.Description, n.Type)
				}

				// new value ref to old value, that is why not allocate new slice.
				// Any changing attributes do not affect category, because changing full slice, not element.
				new.Data[i].Value = o.Value
				break
			}
		}
	}

	*c = new
	return nil
}

// IDVersion returns id and version of the category.
func (c Category) IDVersion() (id int64, version int) {
	s := strings.Split(c.Key, ".")
	if len(s) != 2 {
		return 0, 0
	}

	id, _ = strconv.ParseInt(s[0], 10, 64)
	version, _ = strconv.Atoi(s[1])
	return id, version
}

type TypeValue int

const (
	NilType TypeValue = iota
	StringType
	IntType
	BoolType
	TimeType
)

func (t TypeValue) String() string {
	switch t {
	case StringType:
		return "Core.StringValue"
	case IntType:
		return "Core.IntegerValue"
	case BoolType:
		return "Core.BooleanValue"
	case TimeType:
		return "Core.DateValue"
	default:
		return "NilType"
	}
}

func (t *TypeValue) UnmarshalOscript(b []byte) error {
	if len(b) < 2 || b[0] != '\'' || b[len(b)-1] != '\'' {
		return errTypeValue
	}

	tn := string(b[1 : len(b)-1])
	switch tn {
	case "Core.StringValue":
		*t = StringType
	case "Core.IntegerValue":
		*t = IntType
	case "Core.BooleanValue":
		*t = BoolType
	case "Core.DateValue":
		*t = TimeType
	default:
		return fmt.Errorf("unknown TypeValue \"%s\"", tn)
	}

	return nil
}

func (t TypeValue) MarshalOscript() ([]byte, error) {
	switch t {
	case StringType:
		return []byte("'Core.StringValue'"), nil
	case IntType:
		return []byte("'Core.IntegerValue'"), nil
	case BoolType:
		return []byte("'Core.BooleanValue'"), nil
	case TimeType:
		return []byte("'Core.DateValue'"), nil
	default: // NilType
		return nil, errNilTypeValue
	}
}

// Set sets values.
func (c *Category) Set(v ...NameValueType) error {
	if c == nil {
		return errCategory
	}

	indAttr := make([]int, 0, len(v))
	// TODO: need optimization, using maps to find value
loop:
	for j, vv := range v {
		for i, cv := range c.Data {
			if cv.Description == vv.Name {
				if vv.Type != NilType && cv.Type != vv.Type {
					return fmt.Errorf("invalid type attribute \"%s\" \"%s\"", vv.Name, vv.Type)
				}

				indAttr = append(indAttr, i)
				continue loop
			}
		}

		// not found attribute, skip
		if j == len(v)-1 {
			v = v[:j]
		} else {
			v = append(v[:j], v[j+1:]...)
		}
	}

	for i, attr := range indAttr {
		c.Data[attr].Value = v[i].Value
	}
	return nil
}

type NameValueType struct {
	Name  string
	Value []interface{}
	Type  TypeValue
}

func AttrString(name string, v string) NameValueType {
	return NameValueType{name, []interface{}{v}, StringType}
}

func AttrInt(name string, v int) NameValueType {
	return NameValueType{name, []interface{}{v}, IntType}
}

func AttrBool(name string, v bool) NameValueType {
	return NameValueType{name, []interface{}{v}, BoolType}
}

func AttrTime(name string, v time.Time) NameValueType {
	return NameValueType{name, []interface{}{v}, TimeType}
}

func AttrNil(name string) NameValueType {
	return NameValueType{name, []interface{}{nil}, NilType}
}

func (c *Category) attr(desc string, t TypeValue) ([]interface{}, error) {
	for _, cv := range c.Data {
		if cv.Description == desc {
			switch cv.Type {
			case StringType:
				return cv.Value, nil

			case IntType:
				return cv.Value, nil

			case BoolType:
				return cv.Value, nil

			case TimeType:
				return cv.Value, nil

			default:
				return nil, fmt.Errorf("invalid type attribute \"%s\" \"%s\"", desc, t)
			}
		}
	}

	return nil, fmt.Errorf("not found attribute \"%s\"", desc)
}

// String returns string value.
func (c *Category) String(name string, v *string) error {
	if c == nil {
		return errCategory
	}

	value, err := c.attr(name, StringType)
	if err != nil {
		return err
	}

	ct, ok := value[0].(string)
	if !ok {
		return fmt.Errorf("failed cast to string")
	}
	*v = ct

	return nil
}

// Int returns int value.
func (c *Category) Int(name string, v *int) error {
	if c == nil {
		return errCategory
	}

	value, err := c.attr(name, IntType)
	if err != nil {
		return err
	}

	ct, ok := value[0].(int)
	if !ok {
		return fmt.Errorf("failed cast to int")
	}
	*v = ct

	return nil
}

// Bool returns bool value.
func (c *Category) Bool(name string, v *bool) error {
	if c == nil {
		return errCategory
	}

	value, err := c.attr(name, BoolType)
	if err != nil {
		return err
	}

	ct, ok := value[0].(bool)
	if !ok {
		return fmt.Errorf("failed cast to bool")
	}
	*v = ct

	return nil
}

// Time returns time value.
func (c *Category) Time(name string, v *time.Time) error {
	if c == nil {
		return errCategory
	}

	value, err := c.attr(name, TimeType)
	if err != nil {
		return err
	}

	ct, ok := value[0].(time.Time)
	if !ok {
		return fmt.Errorf("failed cast to time.Time")
	}
	*v = ct

	return nil
}

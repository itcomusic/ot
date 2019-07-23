package ot

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/itcomusic/ot/pkg/oscript"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeValue_UnmarshalOscript(t *testing.T) {
	for _, tt := range []struct {
		in  []byte
		err error
		exp TypeValue
	}{
		{in: []byte("'Core.StringValue'"), exp: StringType},
		{in: []byte("'Core.IntegerValue'"), exp: IntType},
		{in: []byte("'Core.BooleanValue'"), exp: BoolType},
		{in: []byte("'Core.DateValue'"), exp: TimeType},
		{in: []byte("'Core.Unknown'"), err: fmt.Errorf("unknown TypeValue \"Core.Unknown\"")},
		{in: []byte("1"), err: errTypeValue},
	} {
		var got TypeValue
		err := oscript.Unmarshal(tt.in, &got)
		if tt.err != nil {
			assert.Equal(t, tt.err, err)
			continue
		}

		require.Nil(t, err)
		assert.Equal(t, tt.exp, got)
	}
}

func TestTypeValue_MarshalOscript(t *testing.T) {
	for _, tt := range []struct {
		in  TypeValue
		exp []byte
		err error
	}{
		{in: StringType, exp: []byte("'Core.StringValue'")},
		{in: IntType, exp: []byte("'Core.IntegerValue'")},
		{in: BoolType, exp: []byte("'Core.BooleanValue'")},
		{in: TimeType, exp: []byte("'Core.DateValue'")},
		{err: &oscript.MarshalerError{Type: reflect.TypeOf(NilType), Err: errNilTypeValue}, in: NilType},
	} {

		b, err := oscript.Marshal(tt.in)
		if tt.err != nil {
			assert.Equal(t, tt.err, err)
			continue
		}

		require.Nil(t, err)
		assert.Equal(t, tt.exp, b)
	}
}

func TestCategory_Upgrade(t *testing.T) {
	t.Parallel()

	testdata := []struct {
		node  *Node
		cat   Category
		value []interface{}
	}{
		{
			node: &Node{
				Metadata: Metadata{
					Categories: []Category{
						{
							DisplayName: "Name",
							Key:         "1.1",
							Type:        "Category",
							Data: []Value{
								{
									Description: "Name",
									Key:         "1.1.2",
									Value:       []interface{}{1, 2, 3},
								},
							},
						},
					},
				},
			},
			cat: Category{
				DisplayName: "Name",
				Key:         "1.2",
				Type:        "Category",
				Data: []Value{
					{
						Description: "Name",
						Key:         "1.2.3",
						Value:       []interface{}{nil},
					},
				},
			},
			value: []interface{}{1, 2, 3},
		}}

	for _, tt := range testdata {
		err := tt.node.Metadata.Categories[0].Upgrade(tt.cat)

		require.Nil(t, err)
		assert.Equal(t, []interface{}{nil}, tt.cat.Data[0].Value)

		assert.Equal(t, tt.cat.DisplayName, tt.node.Metadata.Categories[0].DisplayName)
		assert.Equal(t, tt.cat.Key, tt.node.Metadata.Categories[0].Key)
		assert.Equal(t, tt.cat.Type, tt.node.Metadata.Categories[0].Type)
		assert.Equal(t, tt.value, tt.node.Metadata.Categories[0].Data[0].Value)
	}
}

func TestCategory_Set(t *testing.T) {
	t.Parallel()

	cat := &Category{
		Data: []Value{
			{
				Description: "1",
				Value:       []interface{}{nil},
				Type:        StringType,
			},
			{
				Description: "2",
				Value:       []interface{}{nil},
				Type:        IntType,
			},
			{
				Description: "3",
				Value:       []interface{}{nil},
				Type:        BoolType,
			},
			{
				Description: "4",
				Value:       []interface{}{nil},
				Type:        TimeType,
			},
			{
				Description: "5",
				Value:       []interface{}{"s"},
				Type:        StringType,
			},
		},
	}

	testdata := []struct {
		data nameValueType
		err  error
	}{
		{nameValueType{Name: "1", Value: []interface{}{"1"}, Type: StringType}, nil},
		{nameValueType{Name: "2", Value: []interface{}{1}, Type: IntType}, nil},
		{nameValueType{Name: "3", Value: []interface{}{true}, Type: BoolType}, nil},
		{nameValueType{Name: "4", Value: []interface{}{time.Time{}}, Type: TimeType}, nil},
		{nameValueType{Name: "5", Value: []interface{}{nil}, Type: NilType}, nil},
		{nameValueType{Name: "5", Value: []interface{}{1}, Type: IntType}, fmt.Errorf("invalid type attribute \"%s\" \"%s\"", "5", IntType)},
	}

	for i, tt := range testdata {
		err := cat.Set(tt.data)
		if tt.err != nil {
			assert.Equal(t, tt.err.Error(), err.Error())
			continue
		}
		assert.Equal(t, tt.data.Value, cat.Data[i].Value)
	}
}

func TestCategory_String(t *testing.T) {
	t.Parallel()
	assert.Equal(t, nameValueType{"attr", []interface{}{"s"}, StringType}, AttrString("attr", "s"))
}

func TestCategory_Int(t *testing.T) {
	t.Parallel()
	assert.Equal(t, nameValueType{"attr", []interface{}{1}, IntType}, AttrInt("attr", 1))
}

func TestCategory_Bool(t *testing.T) {
	t.Parallel()
	assert.Equal(t, nameValueType{"attr", []interface{}{true}, BoolType}, AttrBool("attr", true))
}

func TestCategory_Time(t *testing.T) {
	t.Parallel()
	assert.Equal(t, nameValueType{"attr", []interface{}{time.Time{}}, TimeType}, AttrTime("attr", time.Time{}))
}

func TestCategory_Nil(t *testing.T) {
	t.Parallel()
	assert.Equal(t, nameValueType{"attr", []interface{}{nil}, NilType}, AttrNil("attr"))
}

func TestCategory_GetString(t *testing.T) {
	t.Parallel()
	cat := &Category{
		Data: []Value{
			{
				Description: "1",
				Value:       []interface{}{"1"},
				Type:        StringType,
			},
		},
	}

	var got string
	err := cat.String("1", &got)
	require.Nil(t, err)
	assert.Equal(t, "1", got)
}

func TestCategory_GetInt(t *testing.T) {
	t.Parallel()
	cat := &Category{
		Data: []Value{
			{
				Description: "2",
				Value:       []interface{}{1},
				Type:        IntType,
			},
		},
	}

	var got int
	err := cat.Int("2", &got)
	require.Nil(t, err)
	assert.Equal(t, 1, got)
}

func TestCategory_GetBool(t *testing.T) {
	t.Parallel()
	cat := &Category{
		Data: []Value{
			{
				Description: "3",
				Value:       []interface{}{true},
				Type:        BoolType,
			},
		},
	}

	var got bool
	err := cat.Bool("3", &got)
	require.Nil(t, err)
	assert.Equal(t, true, got)
}

func TestCategory_GetTime(t *testing.T) {
	t.Parallel()
	cat := &Category{
		Data: []Value{
			{
				Description: "4",
				Value:       []interface{}{time.Time{}},
				Type:        TimeType,
			},
		},
	}

	got := time.Now()
	err := cat.Time("4", &got)
	require.Nil(t, err)
	assert.Equal(t, time.Time{}, got)
}

package ot

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testNode = &Node{
	ContainerInfo: NodeContainerInfo{
		ChildCount: 1,
		ChildTypes: []string{"30000", "Alias", "Category", "Channel", "Collection", "CompoundDoc", "Discussion", "Document", "Folder", "Generation", "PhysicalItem", "PhysicalItemBox", "PhysicalItemContainer", "Report", "TaskList", "URL", "WFMap"},
	},
	CreateDate:  time.Date(2019, 04, 9, 12, 32, 3, 0, time.UTC),
	CreatedBy:   1000,
	DisplayType: "Package",
	Feature: []Feature{
		{
			Name:         "Name",
			BooleanValue: func() *bool { b := true; return &b }(),
			Type:         "Boolean",
		},
		{
			Name:      "Name",
			DateValue: func() *time.Time { d := time.Date(2019, 04, 22, 17, 00, 01, 0, time.UTC); return &d }(),
			Type:      "Date",
		},
		{
			Name:         "Name",
			IntegerValue: func() *int { i := 1; return &i }(),
			Type:         "Integer",
		},
		{
			Name:      "Name",
			LongValue: func() *float64 { f := 1.1; return &f }(),
			Type:      "Long",
		},
		{
			Name:        "Name",
			StringValue: func() *string { s := "string"; return &s }(),
			Type:        "String",
		},
	},
	ID:           1,
	IsContainer:  true,
	IsReference:  true,
	IsReservable: true,
	IsVersional:  true,
	Metadata: Metadata{
		Categories: []Category{{
			DisplayName: "Name",
			Key:         "1234.5",
			Type:        "Category",
			Data: []Value{
				{
					Description: "String",
					Key:         "1234.5.2",
					Value:       []interface{}{"string"},
					Type:        StringType,
				},
				{
					Description: "Date",
					Key:         "1234.5.3",
					Value:       []interface{}{time.Date(2010, 12, 21, 0, 0, 0, 0, time.UTC)},
					Type:        TimeType,
				},
				{
					Description: "Integer",
					Key:         "1234.5.4",
					Value:       []interface{}{int64(1)},
					Type:        IntType,
				},
				{
					Description: "Boolean",
					Key:         "1234.5.5",
					Value:       []interface{}{true},
					Type:        BoolType,
				},
			},
		},
			{
				DisplayName: "External",
				Key:         "ExternalAtt",
				Type:        "ExternalAtt",
				Data: []Value{
					{
						Description: "The date that this item was originally created (outside of Content Server)",
						Key:         "ExternalCreateDate",
						Value:       []interface{}{nil},
						Type:        TimeType,
					},
					{
						Description: "The account associated with this item in an external system (e.g., OPENTEXT\\jdoe, johndoe@opentext.com, etc.)",
						Key:         "ExternalIdentity",
						Value:       []interface{}{""},
						Type:        StringType,
					},
					{
						Description: "The type of account associated with this item in an external system (e.g., nt_domain, email, etc.)",
						Key:         "ExternalIdentityType",
						Value:       []interface{}{""},
						Type:        StringType,
					},
					{
						Description: "The date that this item was last modified (outside of Content Server)",
						Key:         "ExternalModifyDate",
						Value:       []interface{}{nil},
						Type:        TimeType,
					},
					{
						Description: "The name for the external source of this item (e.g., file_system, exchange_mailbox, etc.)",
						Key:         "ExternalSource",
						Value:       []interface{}{""},
						Type:        StringType,
					},
				},
			}},
	},
	ModifyDate:  time.Date(2019, 4, 9, 12, 32, 8, 0, time.UTC),
	Name:        "Name",
	Parent:      -2,
	PartialData: false,
	Permissions: Permissions{
		See:        true,
		SeeContent: true,
		Modify:     true,
		EditAttr:   true,
		EditPerm:   true,
		DeleteVer:  true,
		Delete:     true,
		Reserve:    true,
		Create:     true,
	},
	Type:     "30000",
	VolumeID: 2,
}

func Test_GetNode(t *testing.T) {
	t.Parallel()

	node, err := session(t, func(r io.Reader, w *bufio.Writer, req map[string]interface{}) {
		assert.Equal(t, fmt.Sprint(map[string]interface{}{
			"_ApiName":      "InvokeService",
			"_UserName":     "u",
			"_UserPassword": "p",
			"ServiceName":   "DocumentManagement",
			"ServiceMethod": "GetNode",
			"Arguments": map[string]interface{}{
				"ID": int64(1),
			},
		}), fmt.Sprint(req))

		bytes, err := ioutil.ReadFile("testdata/get-node")
		require.Nil(t, err)

		w.Write(bytes)
		assert.Nil(t, w.Flush())
	}).GetNode(context.Background(), 1)
	require.Nil(t, err)

	assert.Equal(t, testNode, node)
}

func Test_GetNodeByNickname(t *testing.T) {
	t.Parallel()

	node, err := session(t, func(r io.Reader, w *bufio.Writer, req map[string]interface{}) {
		assert.Equal(t, fmt.Sprint(map[string]interface{}{
			"_ApiName":      "InvokeService",
			"_UserName":     "u",
			"_UserPassword": "p",
			"ServiceName":   "DocumentManagement",
			"ServiceMethod": "GetNodeByNickname",
			"Arguments": map[string]interface{}{
				"nickname": "nick",
			},
		}), fmt.Sprint(req))

		bytes, err := ioutil.ReadFile("testdata/get-node")
		require.Nil(t, err)

		w.Write(bytes)
		assert.Nil(t, w.Flush())
	}).GetNodeByNickname(context.Background(), "nick")
	require.Nil(t, err)

	assert.Equal(t, testNode, node)
}

func Test_GetCategory(t *testing.T) {
	t.Parallel()

	cat, err := session(t, func(r io.Reader, w *bufio.Writer, req map[string]interface{}) {
		assert.Equal(t, fmt.Sprint(map[string]interface{}{
			"_ApiName":      "InvokeService",
			"_UserName":     "u",
			"_UserPassword": "p",
			"ServiceName":   "DocumentManagement",
			"ServiceMethod": "GetCategoryTemplate",
			"Arguments": map[string]interface{}{
				"categoryID": int64(1),
			},
		}), fmt.Sprint(req))

		bytes, err := ioutil.ReadFile("testdata/get-category")
		require.Nil(t, err)

		w.Write(bytes)
		assert.Nil(t, w.Flush())
	}).GetCategory(context.TODO(), 1)
	require.Nil(t, err)

	assert.Equal(t, &Category{
		DisplayName: "Name",
		Key:         "1234.5",
		Type:        "Category",
		Data: []Value{
			Value{
				Description: "String",
				Key:         "1234.5.2",
				Value:       []interface{}{"string"},
				Type:        StringType,
			},
			Value{
				Description: "Date",
				Key:         "1234.5.3",
				Value:       []interface{}{time.Date(2010, 12, 21, 0, 0, 0, 0, time.UTC)},
				Type:        TimeType,
			},
			Value{
				Description: "Integer",
				Key:         "1234.5.4",
				Value:       []interface{}{int64(1)},
				Type:        IntType,
			},
			Value{
				Description: "Boolean",
				Key:         "1234.5.5",
				Value:       []interface{}{true},
				Type:        BoolType,
			},
		},
	}, cat)
}

func Test_UpdateNode(t *testing.T) {

}

func Test_DeleteNode(t *testing.T) {

}

func Test_RenameNode(t *testing.T) {

}

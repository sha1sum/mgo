package json

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"testing"
)

type funcN struct {
	Arg1 int `json:"arg1"`
	Arg2 int `json:"arg2"`
}

type funcs struct {
	Func2 *funcN `json:"$func2"`
	Func1 *funcN `json:"$func1"`
}

type funcsText struct {
	Func1 jsonText `json:"$func1"`
	Func2 jsonText `json:"$func2"`
}

type jsonText struct {
	json string
}

func (jt *jsonText) UnmarshalJSON(data []byte) error {
	jt.json = string(data)
	return nil
}

type nestedText struct {
	F jsonText
	B bool
}

var ext Extension

type keyed string

func decodeKeyed(data []byte) (interface{}, error) {
	return keyed(data), nil
}

type keyedType struct {
	K keyed
	I int
}

type docint int

func init() {
	ext.DecodeFunc("Func1", "$func1")
	ext.DecodeFunc("Func2", "$func2", "arg1", "arg2")
	ext.DecodeFunc("Func3", "$func3", "arg1")

	ext.DecodeKeyed("$key1", decodeKeyed)
	ext.DecodeKeyed("$func3", decodeKeyed)

	ext.EncodeType(docint(0), func(v interface{}) ([]byte, error) {
		s := `{"$docint": ` + strconv.Itoa(int(v.(docint))) + `}`
		return []byte(s), nil
	})
}

type extDecodeTest struct {
	in  string
	ptr interface{}
	out interface{}
	err error
}

var extDecodeTests = []extDecodeTest{
	// Functions.
	{in: `Func1()`, ptr: new(interface{}), out: map[string]interface{}{
		"$func1": map[string]interface{}{},
	}},
	{in: `{"v": Func1()}`, ptr: new(interface{}), out: map[string]interface{}{
		"v": map[string]interface{}{"$func1": map[string]interface{}{}},
	}},
	{in: `Func2(1)`, ptr: new(interface{}), out: map[string]interface{}{
		"$func2": map[string]interface{}{"arg1": float64(1)},
	}},
	{in: `Func2(1, 2)`, ptr: new(interface{}), out: map[string]interface{}{
		"$func2": map[string]interface{}{"arg1": float64(1), "arg2": float64(2)},
	}},
	{in: `Func2(Func1())`, ptr: new(interface{}), out: map[string]interface{}{
		"$func2": map[string]interface{}{"arg1": map[string]interface{}{"$func1": map[string]interface{}{}}},
	}},
	{in: `Func2(1, 2, 3)`, ptr: new(interface{}), err: fmt.Errorf("json: too many arguments for function Func2")},
	{in: `BadFunc()`, ptr: new(interface{}), err: fmt.Errorf("json: unknown function BadFunc")},

	{in: `Func1()`, ptr: new(funcs), out: funcs{Func1: &funcN{}}},
	{in: `Func2(1)`, ptr: new(funcs), out: funcs{Func2: &funcN{Arg1: 1}}},
	{in: `Func2(1, 2)`, ptr: new(funcs), out: funcs{Func2: &funcN{Arg1: 1, Arg2: 2}}},

	{in: `Func2(1, 2, 3)`, ptr: new(funcs), err: fmt.Errorf("json: too many arguments for function Func2")},
	{in: `BadFunc()`, ptr: new(funcs), err: fmt.Errorf("json: unknown function BadFunc")},

	{in: `Func2(1)`, ptr: new(jsonText), out: jsonText{"Func2(1)"}},
	{in: `Func2(1, 2)`, ptr: new(funcsText), out: funcsText{Func2: jsonText{"Func2(1, 2)"}}},
	{in: `{"f": Func2(1, 2), "b": true}`, ptr: new(nestedText), out: nestedText{jsonText{"Func2(1, 2)"}, true}},

	{in: `Func1()`, ptr: new(struct{}), out: struct{}{}},

	// Keyed documents.
	{in: `{"v": {"$key1": 1}}`, ptr: new(interface{}), out: map[string]interface{}{"v": keyed(`{"$key1": 1}`)}},
	{in: `{"k": {"$key1": 1}}`, ptr: new(keyedType), out: keyedType{K: keyed(`{"$key1": 1}`)}},
	{in: `{"i": {"$key1": 1}}`, ptr: new(keyedType), err: &UnmarshalTypeError{"object", reflect.TypeOf(0), 18}},

	// Keyed function documents.
	{in: `{"v": Func3()}`, ptr: new(interface{}), out: map[string]interface{}{"v": keyed(`Func3()`)}},
	{in: `{"k": Func3()}`, ptr: new(keyedType), out: keyedType{K: keyed(`Func3()`)}},
	{in: `{"i": Func3()}`, ptr: new(keyedType), err: &UnmarshalTypeError{"object", reflect.TypeOf(0), 13}},
}

type extEncodeTest struct {
	in  interface{}
	out string
	err error
}

var extEncodeTests = []extEncodeTest{
	{in: docint(13), out: "{\"$docint\":13}\n"},
}

func TestExtensionDecode(t *testing.T) {
	for i, tt := range extDecodeTests {
		var scan scanner
		in := []byte(tt.in)
		if err := checkValid(in, &scan); err != nil {
			if !reflect.DeepEqual(err, tt.err) {
				t.Errorf("#%d: checkValid: %#v", i, err)
				continue
			}
		}
		if tt.ptr == nil {
			continue
		}

		// v = new(right-type)
		v := reflect.New(reflect.TypeOf(tt.ptr).Elem())
		dec := NewDecoder(bytes.NewReader(in))
		dec.Extend(&ext)
		if err := dec.Decode(v.Interface()); !reflect.DeepEqual(err, tt.err) {
			t.Errorf("#%d: %v, want %v", i, err, tt.err)
			continue
		} else if err != nil {
			continue
		}
		if !reflect.DeepEqual(v.Elem().Interface(), tt.out) {
			t.Errorf("#%d: mismatch\nhave: %#+v\nwant: %#+v", i, v.Elem().Interface(), tt.out)
			data, _ := Marshal(v.Elem().Interface())
			t.Logf("%s", string(data))
			data, _ = Marshal(tt.out)
			t.Logf("%s", string(data))
			continue
		}
	}
}

func TestExtensionEncode(t *testing.T) {
	var buf bytes.Buffer
	for i, tt := range extEncodeTests {
		buf.Truncate(0)
		enc := NewEncoder(&buf)
		enc.Extend(&ext)
		err := enc.Encode(tt.in)
		if !reflect.DeepEqual(err, tt.err) {
			t.Errorf("#%d: %v, want %v", i, err, tt.err)
			continue
		}
		if buf.String() != tt.out {
			t.Errorf("#%d: mismatch\nhave: %q\nwant: %q", i, buf.String(), tt.out)
		}
	}
}

// Based on the Go json package.
// Original Copyright 2009 The Go Authors. All rights reserved.
// Modifications Copyright 2009 Michael Stephens.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE and LICENSE.GO files.

package mongo

import (
	"reflect";
	"strings";
	"fmt";
	"os";
	"bytes";
	"time";
	"container/vector";
)

type structBuilder struct {
	val	reflect.Value;

	// if map_ != nil, write val to map_[key] on each change
	map_	*reflect.MapValue;
	key	reflect.Value;
}

var nobuilder *structBuilder

func isfloat(v reflect.Value) bool {
	switch v.(type) {
	case *reflect.FloatValue, *reflect.Float32Value, *reflect.Float64Value:
		return true
	}
	return false;
}

func setfloat(v reflect.Value, f float64) {
	switch v := v.(type) {
	case *reflect.FloatValue:
		v.Set(float(f))
	case *reflect.Float32Value:
		v.Set(float32(f))
	case *reflect.Float64Value:
		v.Set(float64(f))
	}
}

func setint(v reflect.Value, i int64) {
	switch v := v.(type) {
	case *reflect.IntValue:
		v.Set(int(i))
	case *reflect.Int8Value:
		v.Set(int8(i))
	case *reflect.Int16Value:
		v.Set(int16(i))
	case *reflect.Int32Value:
		v.Set(int32(i))
	case *reflect.Int64Value:
		v.Set(int64(i))
	case *reflect.UintValue:
		v.Set(uint(i))
	case *reflect.Uint8Value:
		v.Set(uint8(i))
	case *reflect.Uint16Value:
		v.Set(uint16(i))
	case *reflect.Uint32Value:
		v.Set(uint32(i))
	case *reflect.Uint64Value:
		v.Set(uint64(i))
	}
}

// If updating b.val is not enough to update the original,
// copy a changed b.val out to the original.
func (b *structBuilder) Flush() {
	if b == nil {
		return
	}
	if b.map_ != nil {
		b.map_.SetElem(b.key, b.val)
	}
}

func (b *structBuilder) Int64(i int64) {
	if b == nil {
		return
	}
	v := b.val;
	if isfloat(v) {
		setfloat(v, float64(i))
	} else {
		setint(v, i)
	}
}

func (b *structBuilder) Date(t *time.Time) {
	if b == nil {
		return
	}
	if v, ok := b.val.(*reflect.PtrValue); ok {
		v.PointTo(reflect.Indirect(reflect.NewValue(t)))
	}
}

func (b *structBuilder) Int32(i int32) {
	if b == nil {
		return
	}
	v := b.val;
	if isfloat(v) {
		setfloat(v, float64(i))
	} else {
		setint(v, int64(i))
	}
}

func (b *structBuilder) Float64(f float64) {
	if b == nil {
		return
	}
	v := b.val;
	if isfloat(v) {
		setfloat(v, f)
	} else {
		setint(v, int64(f))
	}
}

func (b *structBuilder) Null()	{}

func (b *structBuilder) String(s string) {
	if b == nil {
		return
	}
	if v, ok := b.val.(*reflect.StringValue); ok {
		v.Set(s)
	}
}

func (b *structBuilder) Regex(regex, options string) {
	// Ignore options for now...
	if b == nil {
		return
	}
	if v, ok := b.val.(*reflect.StringValue); ok {
		v.Set(regex)
	}
}

func (b *structBuilder) Bool(tf bool) {
	if b == nil {
		return
	}
	if v, ok := b.val.(*reflect.BoolValue); ok {
		v.Set(tf)
	}
}

func (b *structBuilder) OID(oid []byte) {
	if b == nil {
		return
	}
	if v, ok := b.val.(*reflect.SliceValue); ok {
		if v.Cap() < 12 {
			nv := reflect.MakeSlice(v.Type().(*reflect.SliceType), 12, 12);
			v.Set(nv);
		}
		for i := 0; i < 12; i++ {
			v.Elem(i).(*reflect.Uint8Value).Set(oid[i])
		}
	}
}

func (b *structBuilder) Array() {
	if b == nil {
		return
	}
	if v, ok := b.val.(*reflect.SliceValue); ok {
		if v.IsNil() {
			v.Set(reflect.MakeSlice(v.Type().(*reflect.SliceType), 0, 8))
		}
	}
}

func (b *structBuilder) Elem(i int) Builder {
	if b == nil || i < 0 {
		return nobuilder
	}
	switch v := b.val.(type) {
	case *reflect.ArrayValue:
		if i < v.Len() {
			return &structBuilder{val: v.Elem(i)}
		}
	case *reflect.SliceValue:
		if i > v.Cap() {
			n := v.Cap();
			if n < 8 {
				n = 8
			}
			for n <= i {
				n *= 2
			}
			nv := reflect.MakeSlice(v.Type().(*reflect.SliceType), v.Len(), n);
			reflect.ArrayCopy(nv, v);
			v.Set(nv);
		}
		if v.Len() <= i && i < v.Cap() {
			v.SetLen(i + 1)
		}
		if i < v.Len() {
			return &structBuilder{val: v.Elem(i)}
		}
	}
	return nobuilder;
}

func (b *structBuilder) Object() {
	if b == nil {
		return
	}
	if v, ok := b.val.(*reflect.PtrValue); ok && v.IsNil() {
		if v.IsNil() {
			v.PointTo(reflect.MakeZero(v.Type().(*reflect.PtrType).Elem()));
			b.Flush();
		}
		b.map_ = nil;
		b.val = v.Elem();
	}
	if v, ok := b.val.(*reflect.MapValue); ok && v.IsNil() {
		v.Set(reflect.MakeMap(v.Type().(*reflect.MapType)))
	}
}

func (b *structBuilder) Key(k string) Builder {
	if b == nil {
		return nobuilder
	}
	switch v := reflect.Indirect(b.val).(type) {
	case *reflect.StructValue:
		t := v.Type().(*reflect.StructType);
		// Case-insensitive field lookup.
		k = strings.ToLower(k);
		for i := 0; i < t.NumField(); i++ {
			if strings.ToLower(t.Field(i).Name) == k {
				return &structBuilder{val: v.Field(i)}
			}
		}
	case *reflect.MapValue:
		t := v.Type().(*reflect.MapType);
		if t.Key() != reflect.Typeof(k) {
			break
		}
		key := reflect.NewValue(k);
		elem := v.Elem(key);
		if elem == nil {
			v.SetElem(key, reflect.MakeZero(t.Elem()));
			elem = v.Elem(key);
		}
		return &structBuilder{val: elem, map_: v, key: key};
	}
	return nobuilder;
}

func Unmarshal(b []byte, val interface{}) (err os.Error) {
	sb := &structBuilder{val: reflect.NewValue(val)};
	err = Parse(bytes.NewBuffer(b[4:len(b)]), sb);
	return;
}

func Marshal(val interface{}) (BSON, os.Error) {
	if val == nil {
		return Null, nil
	}

	switch v := val.(type) {
	case float64:
		return &_Number{v, _Null{}}, nil
	case string:
		return &_String{v, _Null{}}, nil
	case bool:
		return &_Boolean{v, _Null{}}, nil
	case int32:
		return &_Int{v, _Null{}}, nil
	case int64:
		return &_Long{v, _Null{}}, nil
	case int:
		return &_Long{int64(v), _Null{}}, nil
	case *time.Time:
		return &_Date{v, _Null{}}, nil
	}

	var value reflect.Value;
	switch nv := reflect.NewValue(val).(type) {
	case *reflect.PtrValue:
		value = nv.Elem()
	default:
		value = nv
	}

	switch fv := value.(type) {
	case *reflect.StructValue:
		o := &_Object{map[string]BSON{}, _Null{}};
		t := fv.Type().(*reflect.StructType);
		for i := 0; i < t.NumField(); i++ {
			key := strings.ToLower(t.Field(i).Name);
			el, err := Marshal(fv.Field(i).Interface());
			if err != nil {
				return nil, err
			}
			o.value[key] = el;
		}
		return o, nil;
	case *reflect.MapValue:
		o := &_Object{map[string]BSON{}, _Null{}};
		mt := fv.Type().(*reflect.MapType);
		if mt.Key() != reflect.Typeof("") {
			return nil, os.NewError("can't marshall maps with non-string key types")
		}

		keys := fv.Keys();
		for _, k := range keys {
			sk := k.(*reflect.StringValue).Get();
			el, err := Marshal(fv.Elem(k).Interface());
			if err != nil {
				return nil, err
			}
			o.value[sk] = el;
		}
		return o, nil;
	case *reflect.SliceValue:
		a := &_Array{new(vector.Vector), _Null{}};
		for i := 0; i < fv.Len(); i++ {
			el, err := Marshal(fv.Elem(i).Interface());
			if err != nil {
				return nil, err
			}
			a.value.Push(el);
		}
		return a, nil;
	default:
		return nil, os.NewError(fmt.Sprintf("don't know how to marshal %v\n", value.Type()))
	}

	return nil, nil;
}

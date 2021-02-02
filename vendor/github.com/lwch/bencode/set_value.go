package bencode

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

type notfound struct{}

var notfoundType = reflect.TypeOf(notfound{})

func setNumber(n number, key string, v reflect.Value) error {
	if v.Type() == notfoundType {
		return nil
	}
	newNumber := func() reflect.Value {
		value := reflect.New(reflect.TypeOf(0))
		value.Elem().SetInt(n.signed)
		return value.Elem()
	}
	newMap := func() {
		value := reflect.MakeMap(reflect.TypeOf(map[string]interface{}{}))
		value.SetMapIndex(reflect.ValueOf(key), newNumber())
		v.Set(value)
	}
	switch v.Kind() {
	case reflect.Int,
		reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64:
		v.SetInt(n.signed)
	case reflect.Uint,
		reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		v.SetUint(n.unsigned)
	case reflect.Interface:
		if len(key) > 0 {
			if v.IsNil() {
				newMap()
				return nil
			}
			return setNumber(n, key, v.Elem())
		}
		v.Set(newNumber())
	case reflect.Map:
		if v.IsNil() {
			newMap()
			return nil
		}
		v.SetMapIndex(reflect.ValueOf(key), newNumber())
	case reflect.Struct:
		value := getDictStructTarget(v, key, reflect.TypeOf(0))
		return setNumber(n, "", value)
	default:
		return fmt.Errorf("can not set number value to variable of type %s", v.Type().String())
	}
	return nil
}

func setString(str, key string, v reflect.Value) error {
	if v.Type() == notfoundType {
		return nil
	}
	newString := func() reflect.Value {
		value := reflect.New(reflect.TypeOf(""))
		value.Elem().SetString(str)
		return value.Elem()
	}
	newMap := func() {
		value := reflect.MakeMap(reflect.TypeOf(map[string]interface{}{}))
		value.SetMapIndex(reflect.ValueOf(key), newString())
		v.Set(value)
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(str)
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("can not set string value to variable of type: %s", v.Type().String())
		}
		v.SetBytes([]byte(str))
	case reflect.Array:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("can not set string value to variable of type: %s", v.Type().String())
		}
		data := []byte(str)
		n := len(data)
		if n > v.Len() {
			n = v.Len()
		}
		for i := 0; i < n; i++ {
			v.Index(i).SetUint(uint64(data[i]))
		}
	case reflect.Interface:
		if len(key) > 0 {
			if v.IsNil() {
				newMap()
				return nil
			}
			return setString(str, key, v.Elem())
		}
		if v.IsNil() {
			v.Set(newString())
			return nil
		} else if v.Elem().Kind() != reflect.String {
			v.Set(newString())
			return nil
		}
		v.Elem().SetString(str)
	case reflect.Map:
		if v.IsNil() {
			newMap()
			return nil
		}
		v.SetMapIndex(reflect.ValueOf(key), newString())
	case reflect.Struct:
		value := getDictStructTarget(v, key, reflect.TypeOf(""))
		return setString(str, "", value)
	default:
		return fmt.Errorf("can not set string value to variable of type %s", v.Type().String())
	}
	return nil
}

func getDictStructTarget(v reflect.Value, key string, notfound reflect.Type) reflect.Value {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		kField := t.Field(i)
		if kField.Tag.Get("bencode") == key {
			return v.Field(i)
		}
	}
	for i := 0; i < t.NumField(); i++ {
		kField := t.Field(i)
		if strings.ToLower(kField.Name) == key {
			return v.Field(i)
		}
		if kField.Anonymous {
			vField := v.Field(i)
			return getDictStructTarget(vField, key, notfound)
		}
	}
	return reflect.New(notfound).Elem()
}

func appendNumber(n number, slice reflect.Value) (reflect.Value, error) {
	v := reflect.New(reflect.TypeOf(0))
	v.Elem().SetInt(n.signed)
	return reflect.Append(slice, v.Elem()), nil
}

func appendString(str string, slice reflect.Value) (reflect.Value, error) {
	v := reflect.New(reflect.TypeOf(""))
	v.Elem().SetString(str)
	return reflect.Append(slice, v.Elem()), nil
}

func appendDict(r io.Reader, slice reflect.Value) (reflect.Value, error) {
	return slice, errors.New("not supported dict in list")
}

func appendList(r io.Reader, slice reflect.Value) (reflect.Value, error) {
	return slice, errors.New("not supported list in list")
}

func convertInt(v reflect.Value, t reflect.Type) reflect.Value {
	var ret reflect.Value
	switch t.Kind() {
	case reflect.Int, reflect.Interface:
		return v
	case reflect.Int8:
		ret = reflect.New(reflect.TypeOf(int8(0)))
		ret.Elem().SetInt(v.Int())
	case reflect.Int16:
		ret = reflect.New(reflect.TypeOf(int16(0)))
		ret.Elem().SetInt(v.Int())
	case reflect.Int32:
		ret = reflect.New(reflect.TypeOf(int32(0)))
		ret.Elem().SetInt(v.Int())
	case reflect.Int64:
		ret = reflect.New(reflect.TypeOf(int64(0)))
		ret.Elem().SetInt(v.Int())
	case reflect.Uint:
		ret = reflect.New(reflect.TypeOf(uint(0)))
		ret.Elem().SetUint(uint64(v.Int()))
	case reflect.Uint8:
		ret = reflect.New(reflect.TypeOf(uint8(0)))
		ret.Elem().SetUint(uint64(v.Int()))
	case reflect.Uint16:
		ret = reflect.New(reflect.TypeOf(uint16(0)))
		ret.Elem().SetUint(uint64(v.Int()))
	case reflect.Uint32:
		ret = reflect.New(reflect.TypeOf(uint32(0)))
		ret.Elem().SetUint(uint64(v.Int()))
	case reflect.Uint64:
		ret = reflect.New(reflect.TypeOf(uint64(0)))
		ret.Elem().SetUint(uint64(v.Int()))
	}
	return ret.Elem()
}

func convertString(v reflect.Value, t reflect.Type) (reflect.Value, error) {
	var ret reflect.Value
	switch t.Kind() {
	case reflect.String, reflect.Interface:
		return v, nil
	case reflect.Slice:
		if t.Elem().Kind() != reflect.Uint8 {
			return ret, fmt.Errorf("can not set string value to variable of type %s", t.String())
		}
		str := []byte(v.String())
		ret = reflect.MakeSlice(t, len(str), len(str))
		for i, ch := range str {
			ret.Index(i).SetUint(uint64(ch))
		}
	case reflect.Array:
		if t.Elem().Kind() != reflect.Uint8 {
			return ret, fmt.Errorf("can not set string value to variable of type %s", t.String())
		}
		str := []byte(v.String())
		n := len(str)
		if n > v.Len() {
			n = v.Len()
		}
		ret = v
		for i := 0; i < n; i++ {

		}
	}
	return ret, nil
}

func setList(v reflect.Value, slice reflect.Value) error {
	if v.Type() == notfoundType {
		return nil
	}
	if slice.Len() == 0 {
		return nil
	}
	switch v.Kind() {
	case reflect.Slice:
		if v.Type() == slice.Type() {
			v.Set(slice)
			return nil
		}
		value := reflect.MakeSlice(v.Type(), 0, 0)
		switch reflect.TypeOf(slice.Index(0).Interface()).Kind() {
		case reflect.Int:
			for i := 0; i < slice.Len(); i++ {
				iv := reflect.ValueOf(slice.Index(i).Interface())
				n := convertInt(iv, v.Type().Elem())
				value = reflect.Append(value, n)
			}
		case reflect.String:
			for i := 0; i < slice.Len(); i++ {
				iv := reflect.ValueOf(slice.Index(i).Interface())
				s, err := convertString(iv, v.Type().Elem())
				if err != nil {
					return err
				}
				value = reflect.Append(value, s)
			}
		}
		v.Set(value)
	case reflect.Array:
		_len := slice.Len()
		if _len > v.Len() {
			_len = v.Len()
		}
		switch reflect.TypeOf(slice.Index(0).Interface()).Kind() {
		case reflect.Int:
			for i := 0; i < _len; i++ {
				iv := reflect.ValueOf(slice.Index(i).Interface())
				n := convertInt(iv, v.Type().Elem())
				v.Index(i).Set(n)
			}
		case reflect.String:
			if v.Type().Elem().Kind() == reflect.Array {
				// [m][n]byte
				if v.Type().Elem().Elem().Kind() != reflect.Uint8 {
					return fmt.Errorf("can not set string value to variable of type %s", v.Type().Elem().String())
				}
				for i := 0; i < _len; i++ {
					target := v.Index(i)
					vi := reflect.ValueOf(slice.Index(i).Interface())
					str := []byte(vi.String())
					n := len(str)
					if n > target.Len() {
						n = target.Len()
					}
					for j := 0; j < n; j++ {
						target.Index(j).SetUint(uint64(str[j]))
					}
				}
				return nil
			}
			for i := 0; i < _len; i++ {
				iv := reflect.ValueOf(slice.Index(i).Interface())
				s, err := convertString(iv, v.Type().Elem())
				if err != nil {
					return err
				}
				v.Index(i).Set(s)
			}
		}
	case reflect.Interface:
		v.Set(slice)
	default:
		return fmt.Errorf("can not set list value to variable of type %s", v.Type().String())
	}
	return nil
}

package bencode

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
)

// Decoder bencode decoder
type Decoder struct {
	r io.Reader
}

// NewDecoder create decoder from io.Reader
func NewDecoder(r io.Reader) Decoder {
	return Decoder{r: r}
}

// Decode decode data
func (dec Decoder) Decode(data interface{}) error {
	if reflect.ValueOf(data).Kind() != reflect.Ptr {
		return errors.New("input value is not pointer")
	}
	return decode(bufio.NewReader(dec.r), "", reflect.ValueOf(data).Elem())
}

// Decode decode data in raw
func Decode(data []byte, value interface{}) error {
	return NewDecoder(bytes.NewReader(data)).Decode(value)
}

func decode(r *bufio.Reader, key string, v reflect.Value) error {
	ch, err := r.ReadByte()
	if err != nil {
		return err
	}
	switch ch {
	case 'i':
		n, err := parseNumber(r)
		if err != nil {
			return err
		}
		return setNumber(n, key, v)
	case 'd':
		return decodeDict(r, v)
	case 'l':
		return decodeList(r, v)
	default:
		str, err := parseString(r, ch)
		if err != nil {
			return err
		}
		return setString(str, key, v)
	}
}

type number struct {
	signed   int64
	unsigned uint64
}

func parseNumber(r *bufio.Reader) (number, error) {
	var ret number
	var str []byte
	for {
		ch, err := r.ReadByte()
		if err != nil {
			return ret, fmt.Errorf("parse number: %v", err)
		}
		if ch == 'e' {
			ret.signed, err = strconv.ParseInt(string(str), 10, 64)
			if err != nil {
				return ret, fmt.Errorf("can not parse %s to signed number", string(str))
			}
			if str[0] != '-' {
				ret.unsigned, err = strconv.ParseUint(string(str), 10, 64)
				if err != nil {
					return ret, fmt.Errorf("can not parse %s to unsigned number", string(str))
				}
			} else {
				ret.unsigned = uint64(ret.signed)
			}
			return ret, nil
		}
		str = append(str, ch)
	}
}

func parseString(r *bufio.Reader, ch byte) (string, error) {
	var len []byte
	len = append(len, ch)
	for {
		ch, err := r.ReadByte()
		if err != nil {
			return "", fmt.Errorf("parse string: %v", err)
		}
		if ch == ':' {
			size, err := strconv.ParseUint(string(len), 10, 64)
			if err != nil {
				return "", fmt.Errorf("can not parse string size: %s", string(len))
			}
			data := make([]byte, size)
			_, err = io.ReadFull(r, data)
			if err != nil {
				return "", fmt.Errorf("parse string value: %v", err)
			}
			return string(data), nil
		}
		len = append(len, ch)
	}
}

func decodeDict(r *bufio.Reader, v reflect.Value) error {
	for {
		ch, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("decode dict: %v", err)
		}
		if ch == 'e' {
			return nil
		}
		key, err := parseString(r, ch)
		var target reflect.Value
		switch v.Kind() {
		case reflect.Interface, reflect.Map:
			if v.IsNil() {
				if v.Type().Kind() == reflect.Interface {
					value := reflect.MakeMap(reflect.TypeOf(map[string]interface{}{}))
					v.Set(value)
				} else {
					value := reflect.MakeMap(v.Type())
					v.Set(value)
				}
			}
			target = v
		case reflect.Struct:
			target = getDictStructTarget(v, key, notfoundType)
		}
		err = decode(r, key, target)
		if err != nil {
			return err
		}
	}
}

func decodeList(r *bufio.Reader, v reflect.Value) error {
	slice := reflect.MakeSlice(reflect.TypeOf([]interface{}{}), 0, 0)
	for {
		ch, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("decode dict: %v", err)
		}
		if ch == 'e' {
			return setList(v, slice)
		}
		switch ch {
		case 'i':
			n, err := parseNumber(r)
			if err != nil {
				return err
			}
			slice, err = appendNumber(n, slice)
		case 'd':
			slice, err = appendDict(r, slice)
		case 'l':
			slice, err = appendList(r, slice)
		default:
			str, err := parseString(r, ch)
			if err != nil {
				return err
			}
			slice, err = appendString(str, slice)
		}
		if err != nil {
			return err
		}
	}
}

// func decodeList2Slice(r *bufio.Reader, v reflect.Value) error {
// 	slice := reflect.MakeSlice(v.Type(), 0, 0)
// 	for {
// 		ch, err := r.ReadByte()
// 		if err != nil {
// 			return fmt.Errorf("decode slice: %v", err)
// 		}
// 		if ch == 'e' {
// 			break
// 		}
// 		v := reflect.New(v.Type().Elem())
// 		switch ch {
// 		case 'i':
// 			n, err := decodeNumber(r, v)
// 			if err != nil {
// 				return err
// 			}
// 			if n.isUnsigned {
// 				v.Elem().SetUint(n.unsigned)
// 			} else {
// 				v.Elem().SetInt(n.signed)
// 			}
// 		case 'd':
// 			err = decodeDict(r, v.Elem())
// 			if err != nil {
// 				return err
// 			}
// 		case 'l':
// 			err = decodeList(r, v.Elem())
// 			if err != nil {
// 				return err
// 			}
// 		default:
// 			err = decodeString(r, v.Elem(), ch)
// 			if err != nil {
// 				return err
// 			}
// 		}
// 		slice = reflect.Append(slice, v.Elem())
// 	}
// 	v.Set(slice)
// 	return nil
// }

// func decodeList2Array(r *bufio.Reader, v reflect.Value) error {
// 	i := 0
// 	for {
// 		ch, err := r.ReadByte()
// 		if err != nil {
// 			return fmt.Errorf("decode slice: %v", err)
// 		}
// 		if ch == 'e' {
// 			break
// 		}
// 		target := v.Index(i)
// 		switch ch {
// 		case 'i':
// 			n, err := decodeNumber(r, target)
// 			if err != nil {
// 				return err
// 			}
// 			if n.isUnsigned {
// 				target.SetUint(n.unsigned)
// 			} else {
// 				target.SetInt(n.signed)
// 			}
// 		case 'd':
// 			err = decodeDict(r, target)
// 			if err != nil {
// 				return err
// 			}
// 		case 'l':
// 			err = decodeList(r, target)
// 			if err != nil {
// 				return err
// 			}
// 		default:
// 			err = decodeString(r, target, ch)
// 			if err != nil {
// 				return err
// 			}
// 		}
// 		i++
// 		if i >= v.Len() {
// 			break
// 		}
// 	}
// 	return nil
// }

// func decodeList2Interface(r *bufio.Reader, v reflect.Value) error {
// 	var t reflect.Type
// 	if !v.IsNil() {
// 		t = v.Elem().Type()
// 	}
// 	var slice reflect.Value
// 	for {
// 		ch, err := r.ReadByte()
// 		if err != nil {
// 			return fmt.Errorf("decode slice: %v", err)
// 		}
// 		if ch == 'e' {
// 			break
// 		}
// 		var value reflect.Value
// 		if t.Kind() != reflect.Invalid {
// 			value = reflect.New(t)
// 		}
// 		switch ch {
// 		case 'i':
// 			if !value.IsValid() {
// 				value = reflect.New(reflect.TypeOf(0))
// 			}
// 			n, err := decodeNumber(r, value)
// 			if err != nil {
// 				return err
// 			}
// 			if n.isUnsigned {
// 				value.Elem().SetUint(n.unsigned)
// 			} else {
// 				value.Elem().SetInt(n.signed)
// 			}
// 		case 'd':
// 			err = decodeDict(r, v.Elem())
// 			if err != nil {
// 				return err
// 			}
// 		case 'l':
// 			err = decodeList(r, v.Elem())
// 			if err != nil {
// 				return err
// 			}
// 		default:
// 			err = decodeString(r, v.Elem(), ch)
// 			if err != nil {
// 				return err
// 			}
// 		}
// 		slice = reflect.Append(slice, v.Elem())
// 	}
// 	v.Set(slice)
// 	return nil
// }

// func decodeList(r *bufio.Reader, v reflect.Value) error {
// 	if v.Kind() == reflect.Interface {
// 		return decodeList2Interface(r, v)
// 	}
// 	if v.Kind() == reflect.Slice {
// 		return decodeList2Slice(r, v)
// 	}
// 	if v.Kind() == reflect.Array {
// 		return decodeList2Array(r, v)
// 	}
// 	return fmt.Errorf("can not set list value to variable of type %s", v.Kind().String())
// }

// func setDictNumber(r *bufio.Reader, key string, v reflect.Value) error {
// 	if v.Kind() == reflect.Map {
// 		target := getDictTarget(v, key, reflect.TypeOf(0))
// 		n, err := decodeNumber(r, target)
// 		if err != nil {
// 			return err
// 		}
// 		v.SetMapIndex(reflect.ValueOf(key), numberByType(n, target.Type()))
// 		return nil
// 	}
// 	target := getDictTarget(v, key, reflect.TypeOf(0))
// 	n, err := decodeNumber(r, target)
// 	if err != nil {
// 		return err
// 	}
// 	if n.isUnsigned {
// 		target.SetUint(n.unsigned)
// 	} else {
// 		target.SetInt(n.signed)
// 	}
// 	return nil
// }

// func setDictString(r *bufio.Reader, key string, v reflect.Value, ch byte) error {
// 	if v.Kind() == reflect.Map {
// 		target := getDictTarget(v, key, reflect.TypeOf(""))
// 		err := decodeString(r, target, ch)
// 		if err != nil {
// 			return err
// 		}
// 		v.SetMapIndex(reflect.ValueOf(key), target)
// 		return nil
// 	}
// 	target := getDictTarget(v, key, reflect.TypeOf(""))
// 	return decodeString(r, target, ch)
// }

// func setDictDict(r *bufio.Reader, key string, v reflect.Value) error {
// 	if v.Kind() == reflect.Map {
// 		target := getDictTarget(v, key, reflect.TypeOf(map[string]interface{}{}))
// 		err := decodeDict(r, target)
// 		if err != nil {
// 			return err
// 		}
// 		v.SetMapIndex(reflect.ValueOf(key), target)
// 		return nil
// 	}
// 	target := getDictTarget(v, key, reflect.TypeOf(map[string]interface{}{}))
// 	return decodeDict(r, target)
// }

// func setDictList(r *bufio.Reader, key string, v reflect.Value) error {
// 	if v.Kind() == reflect.Map {
// 		target := getDictTarget(v, key, reflect.TypeOf([]interface{}{}))
// 		err := decodeList(r, target)
// 		if err != nil {
// 			return err
// 		}
// 		v.SetMapIndex(reflect.ValueOf(key), target)
// 		return nil
// 	}
// 	target := getDictTarget(v, key, reflect.TypeOf([]interface{}{}))
// 	return decodeDict(r, target)
// }

package bencode

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

// Decoder bencode decoder
type Decoder struct {
	r io.Reader
}

// NewDecoder create decoder from io.Reader
func NewDecoder(r io.Reader) Decoder {
	return Decoder{r}
}

// Decode decode data
func (dec Decoder) Decode(data interface{}) error {
	return decode(bufio.NewReader(dec.r), reflect.ValueOf(data))
}

// Decode decode data in raw
func Decode(data []byte, value interface{}) error {
	return NewDecoder(bytes.NewReader(data)).Decode(value)
}

func decode(r *bufio.Reader, v reflect.Value) error {
	if v.Kind() != reflect.Ptr {
		return errors.New("input value is not pointer")
	}
	ch, _, err := r.ReadRune()
	if err != nil {
		return err
	}
	switch ch {
	case 'i':
		return decodeNumber(r, v.Elem())
	case 'd':
		return decodeDict(r, v.Elem())
	default:
		return decodeString(r, v.Elem(), ch)
	}
}

func decodeNumber(r *bufio.Reader, v reflect.Value) error {
	var str []rune
	for {
		ch, _, err := r.ReadRune()
		if err != nil {
			return fmt.Errorf("decode number: %v", err)
		}
		if ch == 'e' {
			switch v.Kind() {
			case reflect.Int,
				reflect.Int8, reflect.Int16,
				reflect.Int32, reflect.Int64:
				n, err := strconv.ParseInt(string(str), 10, v.Type().Bits())
				if err != nil {
					return fmt.Errorf("can not parse %s to %s value", string(str), v.Kind().String())
				}
				v.SetInt(n)
			case reflect.Uint,
				reflect.Uint8, reflect.Uint16,
				reflect.Uint32, reflect.Uint64:
				n, err := strconv.ParseUint(string(str), 10, v.Type().Bits())
				if err != nil {
					return fmt.Errorf("can not parse %s to %s value", string(str), v.Kind().String())
				}
				v.SetUint(n)
			default:
				return fmt.Errorf("can not set number value to variable of type %s", v.Kind().String())
			}
		}
		str = append(str, ch)
	}
}

func decodeDict(r *bufio.Reader, v reflect.Value) error {
	key := reflect.New(reflect.TypeOf(""))
	for {
		ch, _, err := r.ReadRune()
		if err != nil {
			return fmt.Errorf("decode dict: %v", err)
		}
		if ch == 'e' {
			return nil
		}
		err = decodeString(r, key.Elem(), ch)
		if err != nil {
			return err
		}
		ch, _, err = r.ReadRune()
		switch ch {
		case 'i':
			err = setDictNumber(r, key.Elem().String(), v)
		case 'd':
			err = setDictDict(r, key.Elem().String(), v)
		default:
			err = setDictString(r, key.Elem().String(), v, ch)
		}
		if err != nil {
			return err
		}
	}
}

func decodeString(r *bufio.Reader, v reflect.Value, ch rune) error {
	var len []rune
	len = append(len, ch)
	for {
		ch, _, err := r.ReadRune()
		if err != nil {
			return fmt.Errorf("decode string: %v", err)
		}
		if ch == ':' {
			size, err := strconv.ParseUint(string(len), 10, 64)
			if err != nil {
				return fmt.Errorf("can not parse string size: %s", string(len))
			}
			data := make([]rune, size)
			for i := 0; uint64(i) < size; i++ {
				data[i], _, err = r.ReadRune()
				if err != nil {
					return fmt.Errorf("decode string value: %v", err)
				}
			}
			switch v.Kind() {
			case reflect.String:
				v.SetString(string(data))
			case reflect.Slice, reflect.Array:
				if v.Type().ConvertibleTo(bytesType) {
					v.SetBytes([]byte(string(data)))
					return nil
				}
				min := size
				if uint64(v.Len()) < min {
					min = uint64(v.Len())
				}
				bt := []byte(string(data))
				for i := 0; i < int(min); i++ {
					if v.Index(i).Kind() != reflect.Uint8 {
						return fmt.Errorf("can not set string value to variable of type %s", v.Kind().String())
					}
					v.Index(i).SetUint(uint64(bt[i]))
				}
			default:
				return fmt.Errorf("can not set string value to variable of type %s", v.Kind().String())
			}
			return nil
		}
		len = append(len, ch)
	}
}

func setDictDict(r *bufio.Reader, key string, v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		kField := t.Field(i)
		if kField.Tag.Get("bencode") == key {
			return decodeDict(r, v.Field(i))
		}
	}
	for i := 0; i < t.NumField(); i++ {
		kField := t.Field(i)
		if strings.ToLower(kField.Name) == key {
			return decodeDict(r, v.Field(i))
		}
	}
	return nil
}

func setDictNumber(r *bufio.Reader, key string, v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		kField := t.Field(i)
		if kField.Tag.Get("bencode") == key {
			return decodeNumber(r, v.Field(i))
		}
	}
	for i := 0; i < t.NumField(); i++ {
		kField := t.Field(i)
		if strings.ToLower(kField.Name) == key {
			return decodeNumber(r, v.Field(i))
		}
	}
	return nil
}

func setDictString(r *bufio.Reader, key string, v reflect.Value, ch rune) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		kField := t.Field(i)
		if kField.Tag.Get("bencode") == key {
			return decodeString(r, v.Field(i), ch)
		}
	}
	for i := 0; i < t.NumField(); i++ {
		kField := t.Field(i)
		if strings.ToLower(kField.Name) == key {
			return decodeString(r, v.Field(i), ch)
		}
	}
	return nil
}

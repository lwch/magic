package bencode

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Encoder bencode encoder
type Encoder struct {
	w io.Writer
}

var bytesType = reflect.TypeOf([]byte{})

// NewEncoder create encoder to io.Writer
func NewEncoder(w io.Writer) Encoder {
	return Encoder{w}
}

// Encode encode data
func (enc Encoder) Encode(data interface{}) error {
	return encode(enc.w, reflect.ValueOf(data))
}

// Encode encode data in raw
func Encode(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := NewEncoder(&buf).Encode(data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func int64Str(i int64) string {
	neg := i < 0
	if i < 0 {
		i = -i
	}
	var str []byte
	for i != 0 {
		str = append(str, byte(i%10+'0'))
		i /= 10
	}
	if neg {
		str = append(str, '-')
	}
	len := len(str)
	for i := 0; i < len/2; i++ {
		str[i], str[len-i-1] = str[len-i-1], str[i]
	}
	return string(str)
}

func uint64Str(i uint64) string {
	var str []byte
	for i != 0 {
		str = append(str, byte(i%10+'0'))
		i /= 10
	}
	len := len(str)
	for i := 0; i < len/2; i++ {
		str[i], str[len-i-1] = str[len-i-1], str[i]
	}
	return string(str)
}

func encode(buf io.Writer, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Int,
		reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64:
		_, err := buf.Write([]byte("i" + int64Str(v.Int()) + "e"))
		return err
	case reflect.Uint,
		reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		_, err := buf.Write([]byte("i" + uint64Str(v.Uint()) + "e"))
		return err
	case reflect.String:
		_, err := buf.Write([]byte(uint64Str(uint64(v.Len())) + ":"))
		if err != nil {
			return err
		}
		_, err = buf.Write([]byte(v.String()))
		return err
	case reflect.Slice:
		_, err := buf.Write([]byte("l"))
		if err != nil {
			return err
		}
		for i := 0; i < v.Len(); i++ {
			err = encode(buf, v.Index(i))
			if err != nil {
				return err
			}
		}
		_, err = buf.Write([]byte("e"))
		return err
	case reflect.Array:
		// if v.Type().ConvertibleTo(bytesType) {
		// 	data := v.Bytes()
		// 	_, err := buf.Write([]byte(fmt.Sprintf("%d:", len(data))))
		// 	if err != nil {
		// 		return err
		// 	}
		// 	_, err = buf.Write(data)
		// 	return err
		// }
		if v.Index(0).Kind() == reflect.Uint8 {
			data := make([]byte, v.Len())
			for i := 0; i < v.Len(); i++ {
				data[i] = byte(v.Index(i).Uint())
			}
			_, err := buf.Write([]byte(fmt.Sprintf("%d:", v.Len())))
			if err != nil {
				return nil
			}
			_, err = buf.Write(data)
			return err
		}
		_, err := buf.Write([]byte("l"))
		if err != nil {
			return err
		}
		for i := 0; i < v.Len(); i++ {
			err = encode(buf, v.Index(i))
			if err != nil {
				return err
			}
		}
		_, err = buf.Write([]byte("e"))
		return err
	case reflect.Interface:
		return encode(buf, v.Elem())
	case reflect.Map:
		_, err := buf.Write([]byte("d"))
		if err != nil {
			return err
		}
		it := v.MapRange()
		for it.Next() {
			err = encode(buf, it.Key())
			if err != nil {
				return err
			}
			err = encode(buf, it.Value())
			if err != nil {
				return err
			}
		}
		_, err = buf.Write([]byte("e"))
		if err != nil {
			return err
		}
	case reflect.Ptr:
		return encode(buf, v.Elem())
	case reflect.Struct:
		t := v.Type()
		_, err := buf.Write([]byte("d"))
		if err != nil {
			return err
		}
		for i := 0; i < t.NumField(); i++ {
			kField := t.Field(i)
			vField := v.Field(i)
			if kField.Anonymous { // inherit struct
				var data bytes.Buffer
				err = encode(&data, vField)
				if err != nil {
					return err
				}
				bt := data.Bytes()
				if bt[0] == 'd' && bt[len(bt)-1] == 'e' {
					buf.Write(bt[1 : len(bt)-1])
				}
				continue
			}
			k := strings.ToLower(kField.Name)
			tag := kField.Tag.Get("bencode")
			if len(tag) > 0 {
				k = tag
			}
			_, err = buf.Write([]byte(fmt.Sprintf("%d:%s", len(k), k)))
			if err != nil {
				return err
			}
			err = encode(buf, vField)
			if err != nil {
				return err
			}
		}
		_, err = buf.Write([]byte("e"))
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("not supported %s value", v.Kind())
	}
	return nil
}

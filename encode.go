package tlv

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

func Marshal(i interface{}, valType uint64) (raw []byte, err error) {
	buf := new(bytes.Buffer)
	err = encode(buf, reflect.ValueOf(i), valType)
	if err != nil {
		return
	}
	raw = buf.Bytes()
	return
}

func WriteBytes(buf *bytes.Buffer, v uint64) (err error) {
	switch {
	case v > math.MaxUint32:
		buf.WriteByte(0xFF)
		err = binary.Write(buf, binary.BigEndian, v)
	case v > math.MaxUint16:
		buf.WriteByte(0xFE)
		err = binary.Write(buf, binary.BigEndian, uint32(v))
	case v > math.MaxUint8-3:
		buf.WriteByte(0xFD)
		err = binary.Write(buf, binary.BigEndian, uint16(v))
	default:
		err = binary.Write(buf, binary.BigEndian, uint8(v))
	}
	return
}

func encodeUint64(buf *bytes.Buffer, v uint64) (err error) {
	switch {
	case v > math.MaxUint32:
		WriteBytes(buf, 8)
		err = binary.Write(buf, binary.BigEndian, v)
	case v > math.MaxUint16:
		WriteBytes(buf, 4)
		err = binary.Write(buf, binary.BigEndian, uint32(v))
	case v > math.MaxUint8:
		WriteBytes(buf, 2)
		err = binary.Write(buf, binary.BigEndian, uint16(v))
	default:
		WriteBytes(buf, 1)
		err = binary.Write(buf, binary.BigEndian, uint8(v))
	}
	return
}

func encodeString(buf *bytes.Buffer, v string) (err error) {
	WriteBytes(buf, uint64(len(v)))
	_, err = buf.WriteString(v)
	return
}

func encodeBytes(buf *bytes.Buffer, v []byte) (err error) {
	WriteBytes(buf, uint64(len(v)))
	_, err = buf.Write(v)
	return
}

func Type(v reflect.Value, i int) (u uint64, err error) {
	s := strings.TrimSuffix(v.Type().Field(i).Tag.Get("tlv"), ",-")
	if len(s) == 0 {
		err = errors.New(fmt.Sprintf("type not found: %s %s", v.Type().Name(), v.Type().Field(i).Name))
		return
	}
	u, err = strconv.ParseUint(s, 10, 64)
	return
}

func optional(v reflect.Value, i int) bool {
	return strings.HasSuffix(v.Type().Field(i).Tag.Get("tlv"), ",-")
}

func zero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool() == false
	case reflect.Uint64:
		return v.Uint() == 0
	case reflect.Ptr:
		fallthrough
	case reflect.Slice:
		return v.IsNil()
	case reflect.String:
		return v.String() == ""
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !zero(v.Field(i)) {
				return false
			}
		}
		return true
	}
	return false
}

func encode(buf *bytes.Buffer, value reflect.Value, valType uint64) (err error) {
	if value.Kind() != reflect.Slice {
		WriteBytes(buf, valType)
	}
	switch value.Kind() {
	case reflect.Bool:
		// no length
		WriteBytes(buf, 0)
	case reflect.Uint64:
		err = encodeUint64(buf, value.Uint())
		if err != nil {
			return
		}
	case reflect.Slice:
		switch value.Type().Elem().Kind() {
		case reflect.Uint8:
			WriteBytes(buf, valType)
			err = encodeBytes(buf, value.Bytes())
			if err != nil {
				return
			}
		default:
			for j := 0; j < value.Len(); j++ {
				err = encode(buf, value.Index(j), valType)
				if err != nil {
					return
				}
			}
		}
	case reflect.String:
		err = encodeString(buf, value.String())
		if err != nil {
			return
		}
	case reflect.Ptr:
		value = value.Elem()
		fallthrough
	case reflect.Struct:
		err = encodeStruct(buf, value)
		if err != nil {
			return
		}
	default:
		err = errors.New("invalid type: " + value.Kind().String())
		return
	}
	return
}

func encodeStruct(buf *bytes.Buffer, structValue reflect.Value) (err error) {
	childBuf := new(bytes.Buffer)
	for i := 0; i < structValue.NumField(); i++ {
		fieldValue := structValue.Field(i)
		if optional(structValue, i) && zero(fieldValue) {
			continue
		}
		var valType uint64
		valType, err = Type(structValue, i)
		if err != nil {
			return
		}
		err = encode(childBuf, fieldValue, valType)
		if err != nil {
			return
		}
	}
	WriteBytes(buf, uint64(childBuf.Len()))
	buf.ReadFrom(childBuf)
	return
}

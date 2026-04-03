package proto

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

func appendStringField(b []byte, field protowire.Number, value string) []byte {
	if value == "" {
		return b
	}
	b = protowire.AppendTag(b, field, protowire.BytesType)
	return protowire.AppendString(b, value)
}

func appendBytesField(b []byte, field protowire.Number, value []byte) []byte {
	if len(value) == 0 {
		return b
	}
	b = protowire.AppendTag(b, field, protowire.BytesType)
	return protowire.AppendBytes(b, value)
}

func appendBoolField(b []byte, field protowire.Number, value bool) []byte {
	if !value {
		return b
	}
	b = protowire.AppendTag(b, field, protowire.VarintType)
	return protowire.AppendVarint(b, 1)
}

func appendUint32Field(b []byte, field protowire.Number, value uint32) []byte {
	if value == 0 {
		return b
	}
	b = protowire.AppendTag(b, field, protowire.VarintType)
	return protowire.AppendVarint(b, uint64(value))
}

func appendInt32Field(b []byte, field protowire.Number, value int32) []byte {
	if value == 0 {
		return b
	}
	b = protowire.AppendTag(b, field, protowire.VarintType)
	return protowire.AppendVarint(b, uint64(int64(value)))
}

func appendInt64Field(b []byte, field protowire.Number, value int64) []byte {
	if value == 0 {
		return b
	}
	b = protowire.AppendTag(b, field, protowire.VarintType)
	return protowire.AppendVarint(b, uint64(value))
}

func appendMessageField(b []byte, field protowire.Number, payload []byte) []byte {
	if len(payload) == 0 {
		return b
	}
	b = protowire.AppendTag(b, field, protowire.BytesType)
	return protowire.AppendBytes(b, payload)
}

func consumeField(data []byte) (protowire.Number, protowire.Type, []byte, []byte, error) {
	num, typ, n := protowire.ConsumeTag(data)
	if n < 0 {
		return 0, 0, nil, nil, protowire.ParseError(n)
	}
	m := protowire.ConsumeFieldValue(num, typ, data[n:])
	if m < 0 {
		return 0, 0, nil, nil, protowire.ParseError(m)
	}
	return num, typ, data[n : n+m], data[n+m:], nil
}

func readVarint(data []byte) (uint64, error) {
	v, n := protowire.ConsumeVarint(data)
	if n < 0 {
		return 0, protowire.ParseError(n)
	}
	return v, nil
}

func readBytes(data []byte) ([]byte, error) {
	v, n := protowire.ConsumeBytes(data)
	if n < 0 {
		return nil, protowire.ParseError(n)
	}
	return v, nil
}

func readString(data []byte) (string, error) {
	v, n := protowire.ConsumeString(data)
	if n < 0 {
		return "", protowire.ParseError(n)
	}
	return v, nil
}

func asBool(v uint64) bool {
	return v != 0
}

func asInt32(v uint64) int32 {
	return int32(v)
}

func asInt64(v uint64) int64 {
	return int64(v)
}

func fieldTypeError(field protowire.Number, typ protowire.Type, want protowire.Type) error {
	return fmt.Errorf("field %d has wire type %d, want %d", field, typ, want)
}

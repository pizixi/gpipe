package proto

import (
	"github.com/pizixi/gpipe/internal/pb"
	"google.golang.org/protobuf/encoding/protowire"
)

func marshalUpgradeOffer(m *pb.UpgradeOffer) []byte {
	var b []byte
	b = appendStringField(b, 1, m.TaskID)
	b = appendStringField(b, 2, m.Version)
	b = appendStringField(b, 3, m.Platform)
	b = appendInt64Field(b, 4, m.Size)
	b = appendStringField(b, 5, m.SHA256)
	b = appendStringField(b, 6, m.Signature)
	b = appendUint32Field(b, 7, m.ChunkSize)
	return b
}

func unmarshalUpgradeOffer(data []byte) (*pb.UpgradeOffer, error) {
	m := &pb.UpgradeOffer{}
	for len(data) > 0 {
		num, typ, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1, 2, 3, 5, 6:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			v, readErr := readString(field)
			if readErr != nil {
				return nil, readErr
			}
			switch num {
			case 1:
				m.TaskID = v
			case 2:
				m.Version = v
			case 3:
				m.Platform = v
			case 5:
				m.SHA256 = v
			case 6:
				m.Signature = v
			}
		case 4, 7:
			if typ != protowire.VarintType {
				return nil, fieldTypeError(num, typ, protowire.VarintType)
			}
			v, readErr := readVarint(field)
			if readErr != nil {
				return nil, readErr
			}
			if num == 4 {
				m.Size = asInt64(v)
			} else {
				m.ChunkSize = uint32(v)
			}
		}
		data = rest
	}
	return m, nil
}

func marshalUpgradeChunk(m *pb.UpgradeChunk) []byte {
	var b []byte
	b = appendStringField(b, 1, m.TaskID)
	b = appendInt64Field(b, 2, m.Offset)
	b = appendBytesField(b, 3, m.Data)
	b = appendStringField(b, 4, m.SHA256)
	b = appendBoolField(b, 5, m.EOF)
	return b
}

func unmarshalUpgradeChunk(data []byte) (*pb.UpgradeChunk, error) {
	m := &pb.UpgradeChunk{}
	for len(data) > 0 {
		num, typ, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1, 4:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			v, readErr := readString(field)
			if readErr != nil {
				return nil, readErr
			}
			if num == 1 {
				m.TaskID = v
			} else {
				m.SHA256 = v
			}
		case 2, 5:
			if typ != protowire.VarintType {
				return nil, fieldTypeError(num, typ, protowire.VarintType)
			}
			v, readErr := readVarint(field)
			if readErr != nil {
				return nil, readErr
			}
			if num == 2 {
				m.Offset = asInt64(v)
			} else {
				m.EOF = asBool(v)
			}
		case 3:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			m.Data, err = readBytes(field)
			if err != nil {
				return nil, err
			}
		}
		data = rest
	}
	return m, nil
}

func marshalUpgradeStatusReport(m *pb.UpgradeStatusReport) []byte {
	var b []byte
	b = appendStringField(b, 1, m.TaskID)
	b = appendStringField(b, 2, m.State)
	b = appendInt64Field(b, 3, m.Offset)
	b = appendStringField(b, 4, m.Error)
	return b
}

func unmarshalUpgradeStatusReport(data []byte) (*pb.UpgradeStatusReport, error) {
	m := &pb.UpgradeStatusReport{}
	for len(data) > 0 {
		num, typ, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1, 2, 4:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			v, readErr := readString(field)
			if readErr != nil {
				return nil, readErr
			}
			switch num {
			case 1:
				m.TaskID = v
			case 2:
				m.State = v
			case 4:
				m.Error = v
			}
		case 3:
			if typ != protowire.VarintType {
				return nil, fieldTypeError(num, typ, protowire.VarintType)
			}
			v, readErr := readVarint(field)
			if readErr != nil {
				return nil, readErr
			}
			m.Offset = asInt64(v)
		}
		data = rest
	}
	return m, nil
}

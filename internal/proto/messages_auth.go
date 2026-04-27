package proto

import (
	"github.com/pizixi/gpipe/internal/pb"

	"google.golang.org/protobuf/encoding/protowire"
)

func marshalLoginReq(m *pb.LoginReq) []byte {
	var b []byte
	b = appendStringField(b, 1, m.Version)
	if m.Username != "" {
		b = appendStringField(b, 2, m.Username)
	}
	if m.Password != "" {
		b = appendStringField(b, 3, m.Password)
	}
	return b
}

func unmarshalLoginReq(data []byte) (*pb.LoginReq, error) {
	msg := &pb.LoginReq{}
	for len(data) > 0 {
		num, typ, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			msg.Version, err = readString(field)
		case 2:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			msg.Username, err = readString(field)
		case 3:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			msg.Password, err = readString(field)
		}
		if err != nil {
			return nil, err
		}
		data = rest
	}
	return msg, nil
}

func marshalRegisterReq(m *pb.RegisterReq) []byte {
	var b []byte
	b = appendStringField(b, 1, m.Username)
	b = appendStringField(b, 2, m.Password)
	return b
}

func unmarshalRegisterReq(data []byte) (*pb.RegisterReq, error) {
	msg := &pb.RegisterReq{}
	for len(data) > 0 {
		num, typ, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			msg.Username, err = readString(field)
		case 2:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			msg.Password, err = readString(field)
		}
		if err != nil {
			return nil, err
		}
		data = rest
	}
	return msg, nil
}

func marshalManagementLoginReq(m *pb.ManagementLoginReq) []byte {
	var b []byte
	b = appendStringField(b, 1, m.Username)
	b = appendStringField(b, 2, m.Password)
	return b
}

func unmarshalManagementLoginReq(data []byte) (*pb.ManagementLoginReq, error) {
	msg := &pb.ManagementLoginReq{}
	for len(data) > 0 {
		num, typ, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			msg.Username, err = readString(field)
		case 2:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			msg.Password, err = readString(field)
		}
		if err != nil {
			return nil, err
		}
		data = rest
	}
	return msg, nil
}

func marshalLoginAck(m *pb.LoginAck) ([]byte, error) {
	var b []byte
	b = appendUint32Field(b, 1, m.PlayerID)
	for _, tunnel := range m.TunnelList {
		payload, err := marshalTunnel(tunnel)
		if err != nil {
			return nil, err
		}
		b = appendMessageField(b, 2, payload)
	}
	b = appendBoolField(b, 3, m.SupportsTunnelRuntimeReport)
	return b, nil
}

func unmarshalLoginAck(data []byte) (*pb.LoginAck, error) {
	msg := &pb.LoginAck{}
	for len(data) > 0 {
		num, typ, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			if typ != protowire.VarintType {
				return nil, fieldTypeError(num, typ, protowire.VarintType)
			}
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.PlayerID = uint32(v)
		case 2:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			raw, err := readBytes(field)
			if err != nil {
				return nil, err
			}
			tunnel, err := unmarshalTunnel(raw)
			if err != nil {
				return nil, err
			}
			msg.TunnelList = append(msg.TunnelList, tunnel)
		case 3:
			if typ != protowire.VarintType {
				return nil, fieldTypeError(num, typ, protowire.VarintType)
			}
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.SupportsTunnelRuntimeReport = asBool(v)
		}
		data = rest
	}
	return msg, nil
}

func marshalTunnelRuntimeReport(m *pb.TunnelRuntimeReport) []byte {
	var b []byte
	b = appendUint32Field(b, 1, m.TunnelID)
	b = appendStringField(b, 2, m.Component)
	b = appendBoolField(b, 3, m.Running)
	b = appendStringField(b, 4, m.Error)
	return b
}

func unmarshalTunnelRuntimeReport(data []byte) (*pb.TunnelRuntimeReport, error) {
	msg := &pb.TunnelRuntimeReport{}
	for len(data) > 0 {
		num, typ, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			if typ != protowire.VarintType {
				return nil, fieldTypeError(num, typ, protowire.VarintType)
			}
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.TunnelID = uint32(v)
		case 2:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			msg.Component, err = readString(field)
			if err != nil {
				return nil, err
			}
		case 3:
			if typ != protowire.VarintType {
				return nil, fieldTypeError(num, typ, protowire.VarintType)
			}
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.Running = asBool(v)
		case 4:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			msg.Error, err = readString(field)
			if err != nil {
				return nil, err
			}
		}
		data = rest
	}
	return msg, nil
}

func marshalManagementLoginAck(m *pb.ManagementLoginAck) []byte {
	var b []byte
	b = appendInt32Field(b, 1, m.Code)
	return b
}

func unmarshalManagementLoginAck(data []byte) (*pb.ManagementLoginAck, error) {
	msg := &pb.ManagementLoginAck{}
	for len(data) > 0 {
		num, typ, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		if num == 1 {
			if typ != protowire.VarintType {
				return nil, fieldTypeError(num, typ, protowire.VarintType)
			}
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.Code = asInt32(v)
		}
		data = rest
	}
	return msg, nil
}

func marshalModifyTunnelNtf(m *pb.ModifyTunnelNtf) ([]byte, error) {
	var b []byte
	b = appendBoolField(b, 1, m.IsDelete)
	if m.Tunnel != nil {
		payload, err := marshalTunnel(m.Tunnel)
		if err != nil {
			return nil, err
		}
		b = appendMessageField(b, 2, payload)
	}
	return b, nil
}

func unmarshalModifyTunnelNtf(data []byte) (*pb.ModifyTunnelNtf, error) {
	msg := &pb.ModifyTunnelNtf{}
	for len(data) > 0 {
		num, typ, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			if typ != protowire.VarintType {
				return nil, fieldTypeError(num, typ, protowire.VarintType)
			}
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.IsDelete = asBool(v)
		case 2:
			if typ != protowire.BytesType {
				return nil, fieldTypeError(num, typ, protowire.BytesType)
			}
			raw, err := readBytes(field)
			if err != nil {
				return nil, err
			}
			msg.Tunnel, err = unmarshalTunnel(raw)
			if err != nil {
				return nil, err
			}
		}
		data = rest
	}
	return msg, nil
}

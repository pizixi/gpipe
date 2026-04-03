package proto

import "github.com/pizixi/gpipe/internal/pb"

func marshalFail(m *pb.Fail) []byte {
	return marshalError(&pb.Error{Number: m.Number, Message: m.Message})
}

func unmarshalFail(data []byte) (*pb.Fail, error) {
	msg, err := unmarshalError(data)
	if err != nil {
		return nil, err
	}
	return &pb.Fail{Number: msg.Number, Message: msg.Message}, nil
}

func marshalError(m *pb.Error) []byte {
	var b []byte
	b = appendInt32Field(b, 1, m.Number)
	b = appendStringField(b, 2, m.Message)
	return b
}

func unmarshalError(data []byte) (*pb.Error, error) {
	msg := &pb.Error{}
	for len(data) > 0 {
		num, _, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.Number = asInt32(v)
		case 2:
			msg.Message, err = readString(field)
			if err != nil {
				return nil, err
			}
		}
		data = rest
	}
	return msg, nil
}

func marshalPing(m *pb.Ping) []byte {
	var b []byte
	b = appendInt64Field(b, 1, m.Ticks)
	return b
}

func unmarshalPing(data []byte) (*pb.Ping, error) {
	msg := &pb.Ping{}
	for len(data) > 0 {
		num, _, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		if num == 1 {
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.Ticks = asInt64(v)
		}
		data = rest
	}
	return msg, nil
}

func marshalPong(m *pb.Pong) []byte {
	return marshalPing(&pb.Ping{Ticks: m.Ticks})
}

func unmarshalPong(data []byte) (*pb.Pong, error) {
	msg, err := unmarshalPing(data)
	if err != nil {
		return nil, err
	}
	return &pb.Pong{Ticks: msg.Ticks}, nil
}

func marshalI2OConnect(m *pb.I2OConnect) []byte {
	var b []byte
	b = appendUint32Field(b, 1, m.TunnelID)
	b = appendUint32Field(b, 2, m.SessionID)
	b = appendUint32Field(b, 3, m.TunnelType)
	b = appendBoolField(b, 4, m.IsTCP)
	b = appendBoolField(b, 5, m.IsCompressed)
	b = appendStringField(b, 6, m.Addr)
	b = appendStringField(b, 7, m.EncryptionMethod)
	b = appendStringField(b, 8, m.EncryptionKey)
	b = appendStringField(b, 9, m.ClientAddr)
	return b
}

func unmarshalI2OConnect(data []byte) (*pb.I2OConnect, error) {
	msg := &pb.I2OConnect{}
	for len(data) > 0 {
		num, _, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.TunnelID = uint32(v)
		case 2:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.SessionID = uint32(v)
		case 3:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.TunnelType = uint32(v)
		case 4:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.IsTCP = asBool(v)
		case 5:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.IsCompressed = asBool(v)
		case 6:
			msg.Addr, err = readString(field)
			if err != nil {
				return nil, err
			}
		case 7:
			msg.EncryptionMethod, err = readString(field)
			if err != nil {
				return nil, err
			}
		case 8:
			msg.EncryptionKey, err = readString(field)
			if err != nil {
				return nil, err
			}
		case 9:
			msg.ClientAddr, err = readString(field)
			if err != nil {
				return nil, err
			}
		}
		data = rest
	}
	return msg, nil
}

func marshalO2IConnect(m *pb.O2IConnect) []byte {
	var b []byte
	b = appendUint32Field(b, 1, m.TunnelID)
	b = appendUint32Field(b, 2, m.SessionID)
	b = appendBoolField(b, 3, m.Success)
	b = appendStringField(b, 4, m.ErrorInfo)
	return b
}

func unmarshalO2IConnect(data []byte) (*pb.O2IConnect, error) {
	msg := &pb.O2IConnect{}
	for len(data) > 0 {
		num, _, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.TunnelID = uint32(v)
		case 2:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.SessionID = uint32(v)
		case 3:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.Success = asBool(v)
		case 4:
			msg.ErrorInfo, err = readString(field)
			if err != nil {
				return nil, err
			}
		}
		data = rest
	}
	return msg, nil
}

func marshalI2OSendData(m *pb.I2OSendData) []byte {
	var b []byte
	b = appendUint32Field(b, 1, m.TunnelID)
	b = appendUint32Field(b, 2, m.SessionID)
	b = appendBytesField(b, 3, m.Data)
	return b
}

func unmarshalI2OSendData(data []byte) (*pb.I2OSendData, error) {
	msg := &pb.I2OSendData{}
	return msg, unmarshalBinaryTriplet(data, &msg.TunnelID, &msg.SessionID, &msg.Data)
}

func marshalO2IRecvData(m *pb.O2IRecvData) []byte {
	var b []byte
	b = appendUint32Field(b, 1, m.TunnelID)
	b = appendUint32Field(b, 2, m.SessionID)
	b = appendBytesField(b, 3, m.Data)
	return b
}

func unmarshalO2IRecvData(data []byte) (*pb.O2IRecvData, error) {
	msg := &pb.O2IRecvData{}
	return msg, unmarshalBinaryTriplet(data, &msg.TunnelID, &msg.SessionID, &msg.Data)
}

func marshalI2ODisconnect(m *pb.I2ODisconnect) []byte {
	var b []byte
	b = appendUint32Field(b, 1, m.TunnelID)
	b = appendUint32Field(b, 2, m.SessionID)
	return b
}

func unmarshalI2ODisconnect(data []byte) (*pb.I2ODisconnect, error) {
	msg := &pb.I2ODisconnect{}
	return msg, unmarshalPair(data, &msg.TunnelID, &msg.SessionID)
}

func marshalO2IDisconnect(m *pb.O2IDisconnect) []byte {
	var b []byte
	b = appendUint32Field(b, 1, m.TunnelID)
	b = appendUint32Field(b, 2, m.SessionID)
	return b
}

func unmarshalO2IDisconnect(data []byte) (*pb.O2IDisconnect, error) {
	msg := &pb.O2IDisconnect{}
	return msg, unmarshalPair(data, &msg.TunnelID, &msg.SessionID)
}

func marshalO2ISendDataResult(m *pb.O2ISendDataResult) []byte {
	var b []byte
	b = appendUint32Field(b, 1, m.TunnelID)
	b = appendUint32Field(b, 2, m.SessionID)
	b = appendUint32Field(b, 3, m.DataLen)
	return b
}

func unmarshalO2ISendDataResult(data []byte) (*pb.O2ISendDataResult, error) {
	msg := &pb.O2ISendDataResult{}
	return msg, unmarshalTripleUint32(data, &msg.TunnelID, &msg.SessionID, &msg.DataLen)
}

func marshalI2ORecvDataResult(m *pb.I2ORecvDataResult) []byte {
	var b []byte
	b = appendUint32Field(b, 1, m.TunnelID)
	b = appendUint32Field(b, 2, m.SessionID)
	b = appendUint32Field(b, 3, m.DataLen)
	return b
}

func unmarshalI2ORecvDataResult(data []byte) (*pb.I2ORecvDataResult, error) {
	msg := &pb.I2ORecvDataResult{}
	return msg, unmarshalTripleUint32(data, &msg.TunnelID, &msg.SessionID, &msg.DataLen)
}

func marshalI2OSendToData(m *pb.I2OSendToData) []byte {
	var b []byte
	b = appendUint32Field(b, 1, m.TunnelID)
	b = appendUint32Field(b, 2, m.SessionID)
	b = appendBytesField(b, 3, m.Data)
	b = appendStringField(b, 4, m.TargetAddr)
	return b
}

func unmarshalI2OSendToData(data []byte) (*pb.I2OSendToData, error) {
	msg := &pb.I2OSendToData{}
	for len(data) > 0 {
		num, _, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.TunnelID = uint32(v)
		case 2:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.SessionID = uint32(v)
		case 3:
			msg.Data, err = readBytes(field)
			if err != nil {
				return nil, err
			}
		case 4:
			msg.TargetAddr, err = readString(field)
			if err != nil {
				return nil, err
			}
		}
		data = rest
	}
	return msg, nil
}

func marshalO2IRecvDataFrom(m *pb.O2IRecvDataFrom) []byte {
	var b []byte
	b = appendUint32Field(b, 1, m.TunnelID)
	b = appendUint32Field(b, 2, m.SessionID)
	b = appendBytesField(b, 3, m.Data)
	b = appendStringField(b, 4, m.RemoteAddr)
	return b
}

func unmarshalO2IRecvDataFrom(data []byte) (*pb.O2IRecvDataFrom, error) {
	msg := &pb.O2IRecvDataFrom{}
	for len(data) > 0 {
		num, _, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.TunnelID = uint32(v)
		case 2:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.SessionID = uint32(v)
		case 3:
			msg.Data, err = readBytes(field)
			if err != nil {
				return nil, err
			}
		case 4:
			msg.RemoteAddr, err = readString(field)
			if err != nil {
				return nil, err
			}
		}
		data = rest
	}
	return msg, nil
}

package proto

import "github.com/pizixi/gpipe/internal/pb"

func marshalTunnelPoint(m *pb.TunnelPoint) []byte {
	var b []byte
	if m != nil {
		b = appendStringField(b, 1, m.Addr)
	}
	return b
}

func unmarshalTunnelPoint(data []byte) (*pb.TunnelPoint, error) {
	msg := &pb.TunnelPoint{}
	for len(data) > 0 {
		num, _, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		if num == 1 {
			msg.Addr, err = readString(field)
			if err != nil {
				return nil, err
			}
		}
		data = rest
	}
	return msg, nil
}

func marshalTunnel(m *pb.Tunnel) ([]byte, error) {
	var b []byte
	if m.Source != nil {
		b = appendMessageField(b, 1, marshalTunnelPoint(m.Source))
	}
	if m.Endpoint != nil {
		b = appendMessageField(b, 2, marshalTunnelPoint(m.Endpoint))
	}
	b = appendUint32Field(b, 3, m.ID)
	b = appendBoolField(b, 4, m.Enabled)
	b = appendUint32Field(b, 5, m.Sender)
	b = appendUint32Field(b, 6, m.Receiver)
	b = appendInt32Field(b, 7, m.TunnelType)
	b = appendStringField(b, 8, m.Password)
	b = appendStringField(b, 9, m.Username)
	b = appendBoolField(b, 10, m.IsCompressed)
	b = appendStringField(b, 11, m.EncryptionMethod)
	for k, v := range m.CustomMapping {
		b = appendMessageField(b, 12, marshalMapEntry(k, v))
	}
	return b, nil
}

func unmarshalTunnel(data []byte) (*pb.Tunnel, error) {
	msg := &pb.Tunnel{CustomMapping: map[string]string{}}
	for len(data) > 0 {
		num, _, field, rest, err := consumeField(data)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			raw, err := readBytes(field)
			if err != nil {
				return nil, err
			}
			msg.Source, err = unmarshalTunnelPoint(raw)
			if err != nil {
				return nil, err
			}
		case 2:
			raw, err := readBytes(field)
			if err != nil {
				return nil, err
			}
			msg.Endpoint, err = unmarshalTunnelPoint(raw)
			if err != nil {
				return nil, err
			}
		case 3:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.ID = uint32(v)
		case 4:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.Enabled = asBool(v)
		case 5:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.Sender = uint32(v)
		case 6:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.Receiver = uint32(v)
		case 7:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.TunnelType = asInt32(v)
		case 8:
			msg.Password, err = readString(field)
			if err != nil {
				return nil, err
			}
		case 9:
			msg.Username, err = readString(field)
			if err != nil {
				return nil, err
			}
		case 10:
			v, err := readVarint(field)
			if err != nil {
				return nil, err
			}
			msg.IsCompressed = asBool(v)
		case 11:
			msg.EncryptionMethod, err = readString(field)
			if err != nil {
				return nil, err
			}
		case 12:
			raw, err := readBytes(field)
			if err != nil {
				return nil, err
			}
			k, v, err := unmarshalMapEntry(raw)
			if err != nil {
				return nil, err
			}
			msg.CustomMapping[k] = v
		}
		data = rest
	}
	return msg, nil
}

func marshalMapEntry(key, value string) []byte {
	var b []byte
	b = appendStringField(b, 1, key)
	b = appendStringField(b, 2, value)
	return b
}

func unmarshalMapEntry(data []byte) (string, string, error) {
	var key, value string
	for len(data) > 0 {
		num, _, field, rest, err := consumeField(data)
		if err != nil {
			return "", "", err
		}
		switch num {
		case 1:
			key, err = readString(field)
			if err != nil {
				return "", "", err
			}
		case 2:
			value, err = readString(field)
			if err != nil {
				return "", "", err
			}
		}
		data = rest
	}
	return key, value, nil
}

func unmarshalPair(data []byte, a, b *uint32) error {
	for len(data) > 0 {
		num, _, field, rest, err := consumeField(data)
		if err != nil {
			return err
		}
		v, err := readVarint(field)
		if err != nil {
			return err
		}
		if num == 1 {
			*a = uint32(v)
		} else if num == 2 {
			*b = uint32(v)
		}
		data = rest
	}
	return nil
}

func unmarshalTripleUint32(data []byte, a, b, c *uint32) error {
	for len(data) > 0 {
		num, _, field, rest, err := consumeField(data)
		if err != nil {
			return err
		}
		v, err := readVarint(field)
		if err != nil {
			return err
		}
		switch num {
		case 1:
			*a = uint32(v)
		case 2:
			*b = uint32(v)
		case 3:
			*c = uint32(v)
		}
		data = rest
	}
	return nil
}

func unmarshalBinaryTriplet(data []byte, a, b *uint32, payload *[]byte) error {
	for len(data) > 0 {
		num, _, field, rest, err := consumeField(data)
		if err != nil {
			return err
		}
		switch num {
		case 1:
			v, err := readVarint(field)
			if err != nil {
				return err
			}
			*a = uint32(v)
		case 2:
			v, err := readVarint(field)
			if err != nil {
				return err
			}
			*b = uint32(v)
		case 3:
			v, err := readBytes(field)
			if err != nil {
				return err
			}
			*payload = append((*payload)[:0], v...)
		}
		data = rest
	}
	return nil
}

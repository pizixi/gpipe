package codec

import (
	"encoding/binary"
	"fmt"

	"github.com/pizixi/gpipe/internal/proto"
)

const frameMagic byte = 33

func Encode(serial int32, message proto.Message) ([]byte, error) {
	msgID, ok := proto.MessageID(message)
	if !ok {
		return nil, fmt.Errorf("message id not found for %T", message)
	}
	payload, err := proto.Encode(message)
	if err != nil {
		return nil, err
	}
	frameLen := 8 + len(payload)
	buf := make([]byte, 5+frameLen)
	buf[0] = frameMagic
	binary.BigEndian.PutUint32(buf[1:5], uint32(frameLen))
	binary.BigEndian.PutUint32(buf[5:9], uint32(serial))
	binary.BigEndian.PutUint32(buf[9:13], msgID)
	copy(buf[13:], payload)
	return buf, nil
}

func TryExtractFrame(buffer []byte, maxSize int) (frame []byte, rest []byte, err error) {
	if len(buffer) == 0 {
		return nil, buffer, nil
	}
	if buffer[0] != frameMagic {
		return nil, nil, fmt.Errorf("bad flag")
	}
	if len(buffer) < 5 {
		return nil, buffer, nil
	}
	frameLen := int(binary.BigEndian.Uint32(buffer[1:5]))
	if frameLen <= 0 || frameLen >= maxSize {
		return nil, nil, fmt.Errorf("message too long")
	}
	if len(buffer) < 5+frameLen {
		return nil, buffer, nil
	}
	return buffer[5 : 5+frameLen], buffer[5+frameLen:], nil
}

func Decode(frame []byte) (int32, proto.Message, error) {
	if len(frame) < 8 {
		return 0, nil, fmt.Errorf("message length is too small")
	}
	serial := int32(binary.BigEndian.Uint32(frame[:4]))
	msgID := binary.BigEndian.Uint32(frame[4:8])
	msg, err := proto.Decode(msgID, frame[8:])
	if err != nil {
		return 0, nil, err
	}
	return serial, msg, nil
}

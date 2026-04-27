package proto

import (
	"fmt"

	"github.com/pizixi/gpipe/internal/pb"
)

const (
	MsgClientServerLoginReq            uint32 = 1001
	MsgServerClientLoginAck            uint32 = 1002
	MsgClientServerRegisterReq         uint32 = 1003
	MsgClientServerManagementLoginReq  uint32 = 1005
	MsgServerClientManagementLoginAck  uint32 = 1006
	MsgServerClientModifyTunnelNtf     uint32 = 1008
	MsgClientServerTunnelRuntimeReport uint32 = 1009
	MsgGenericSuccess                  uint32 = 150001
	MsgGenericFail                     uint32 = 150002
	MsgGenericError                    uint32 = 150003
	MsgGenericPing                     uint32 = 150004
	MsgGenericPong                     uint32 = 150005
	MsgGenericI2OConnect               uint32 = 150006
	MsgGenericO2IConnect               uint32 = 150007
	MsgGenericI2OSendData              uint32 = 150008
	MsgGenericO2IRecvData              uint32 = 150009
	MsgGenericI2ODisconnect            uint32 = 150010
	MsgGenericO2IDisconnect            uint32 = 150011
	MsgGenericO2ISendDataResult        uint32 = 150012
	MsgGenericI2ORecvDataResult        uint32 = 150013
	MsgGenericI2OSendToData            uint32 = 150014
	MsgGenericO2IRecvDataFrom          uint32 = 150015
)

type Message interface{}

func MessageID(message Message) (uint32, bool) {
	switch message.(type) {
	case *pb.LoginReq:
		return MsgClientServerLoginReq, true
	case *pb.RegisterReq:
		return MsgClientServerRegisterReq, true
	case *pb.ManagementLoginReq:
		return MsgClientServerManagementLoginReq, true
	case *pb.TunnelRuntimeReport:
		return MsgClientServerTunnelRuntimeReport, true
	case *pb.LoginAck:
		return MsgServerClientLoginAck, true
	case *pb.ManagementLoginAck:
		return MsgServerClientManagementLoginAck, true
	case *pb.ModifyTunnelNtf:
		return MsgServerClientModifyTunnelNtf, true
	case *pb.Success:
		return MsgGenericSuccess, true
	case *pb.Fail:
		return MsgGenericFail, true
	case *pb.Error:
		return MsgGenericError, true
	case *pb.Ping:
		return MsgGenericPing, true
	case *pb.Pong:
		return MsgGenericPong, true
	case *pb.I2OConnect:
		return MsgGenericI2OConnect, true
	case *pb.O2IConnect:
		return MsgGenericO2IConnect, true
	case *pb.I2OSendData:
		return MsgGenericI2OSendData, true
	case *pb.O2IRecvData:
		return MsgGenericO2IRecvData, true
	case *pb.I2ODisconnect:
		return MsgGenericI2ODisconnect, true
	case *pb.O2IDisconnect:
		return MsgGenericO2IDisconnect, true
	case *pb.O2ISendDataResult:
		return MsgGenericO2ISendDataResult, true
	case *pb.I2ORecvDataResult:
		return MsgGenericI2ORecvDataResult, true
	case *pb.I2OSendToData:
		return MsgGenericI2OSendToData, true
	case *pb.O2IRecvDataFrom:
		return MsgGenericO2IRecvDataFrom, true
	default:
		return 0, false
	}
}

func Encode(message Message) ([]byte, error) {
	switch m := message.(type) {
	case *pb.LoginReq:
		return marshalLoginReq(m), nil
	case *pb.RegisterReq:
		return marshalRegisterReq(m), nil
	case *pb.ManagementLoginReq:
		return marshalManagementLoginReq(m), nil
	case *pb.TunnelRuntimeReport:
		return marshalTunnelRuntimeReport(m), nil
	case *pb.LoginAck:
		return marshalLoginAck(m)
	case *pb.ManagementLoginAck:
		return marshalManagementLoginAck(m), nil
	case *pb.ModifyTunnelNtf:
		return marshalModifyTunnelNtf(m)
	case *pb.Success:
		return nil, nil
	case *pb.Fail:
		return marshalFail(m), nil
	case *pb.Error:
		return marshalError(m), nil
	case *pb.Ping:
		return marshalPing(m), nil
	case *pb.Pong:
		return marshalPong(m), nil
	case *pb.I2OConnect:
		return marshalI2OConnect(m), nil
	case *pb.O2IConnect:
		return marshalO2IConnect(m), nil
	case *pb.I2OSendData:
		return marshalI2OSendData(m), nil
	case *pb.O2IRecvData:
		return marshalO2IRecvData(m), nil
	case *pb.I2ODisconnect:
		return marshalI2ODisconnect(m), nil
	case *pb.O2IDisconnect:
		return marshalO2IDisconnect(m), nil
	case *pb.O2ISendDataResult:
		return marshalO2ISendDataResult(m), nil
	case *pb.I2ORecvDataResult:
		return marshalI2ORecvDataResult(m), nil
	case *pb.I2OSendToData:
		return marshalI2OSendToData(m), nil
	case *pb.O2IRecvDataFrom:
		return marshalO2IRecvDataFrom(m), nil
	default:
		return nil, nil
	}
}

func Decode(msgID uint32, payload []byte) (Message, error) {
	switch msgID {
	case MsgClientServerLoginReq:
		return unmarshalLoginReq(payload)
	case MsgClientServerRegisterReq:
		return unmarshalRegisterReq(payload)
	case MsgClientServerManagementLoginReq:
		return unmarshalManagementLoginReq(payload)
	case MsgClientServerTunnelRuntimeReport:
		return unmarshalTunnelRuntimeReport(payload)
	case MsgServerClientLoginAck:
		return unmarshalLoginAck(payload)
	case MsgServerClientManagementLoginAck:
		return unmarshalManagementLoginAck(payload)
	case MsgServerClientModifyTunnelNtf:
		return unmarshalModifyTunnelNtf(payload)
	case MsgGenericSuccess:
		return &pb.Success{}, nil
	case MsgGenericFail:
		return unmarshalFail(payload)
	case MsgGenericError:
		return unmarshalError(payload)
	case MsgGenericPing:
		return unmarshalPing(payload)
	case MsgGenericPong:
		return unmarshalPong(payload)
	case MsgGenericI2OConnect:
		return unmarshalI2OConnect(payload)
	case MsgGenericO2IConnect:
		return unmarshalO2IConnect(payload)
	case MsgGenericI2OSendData:
		return unmarshalI2OSendData(payload)
	case MsgGenericO2IRecvData:
		return unmarshalO2IRecvData(payload)
	case MsgGenericI2ODisconnect:
		return unmarshalI2ODisconnect(payload)
	case MsgGenericO2IDisconnect:
		return unmarshalO2IDisconnect(payload)
	case MsgGenericO2ISendDataResult:
		return unmarshalO2ISendDataResult(payload)
	case MsgGenericI2ORecvDataResult:
		return unmarshalI2ORecvDataResult(payload)
	case MsgGenericI2OSendToData:
		return unmarshalI2OSendToData(payload)
	case MsgGenericO2IRecvDataFrom:
		return unmarshalO2IRecvDataFrom(payload)
	default:
		return nil, fmt.Errorf("unknown message id: %d", msgID)
	}
}

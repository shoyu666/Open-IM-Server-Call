package rtc

import (
	"Open_IM/pkg/common/config"
	"Open_IM/pkg/common/constant"
	"Open_IM/pkg/common/log"
	"Open_IM/pkg/grpc-etcdv3/getcdv3"
	"Open_IM/pkg/proto/rtc"
	pbRtc "Open_IM/pkg/proto/rtc"
	"Open_IM/pkg/utils"
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/livekit/protocol/auth"
	"google.golang.org/grpc"
	"net"
	"strconv"
	"strings"
	"time"
)

type rtcLiveKit struct {
	rpcPort         int
	rpcRegisterName string
	etcdSchema      string
	etcdAddr        []string
	LiveURL         string
	signalTimeout   string
	key             string
	secret          string
}

func NewRpcLiveKitServer(port int) *rtcLiveKit {
	log.NewPrivateLog(constant.LogFileName)
	rc := rtcLiveKit{
		rpcPort:         port,
		etcdSchema:      config.Config.Etcd.EtcdSchema,
		etcdAddr:        config.Config.Etcd.EtcdAddr,
		rpcRegisterName: config.Config.RpcRegisterName.OpenImRealTimeCommName,
		LiveURL:         config.Config.Rtc.LiveURL,
		signalTimeout:   config.Config.Rtc.SignalTimeout,
		key:             config.Config.Rtc.Key,
		secret:          config.Config.Rtc.Secret,
	}
	return &rc
}

func (rpc *rtcLiveKit) Run() {
	log.Info("", "rpcChat init...")
	listenIP := ""
	if config.Config.ListenIP == "" {
		listenIP = "0.0.0.0"
	} else {
		listenIP = config.Config.ListenIP
	}
	address := listenIP + ":" + strconv.Itoa(rpc.rpcPort)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic("listening err:" + err.Error() + rpc.rpcRegisterName)
	}
	log.Info("", "listen network success, address ", address)

	srv := grpc.NewServer()
	defer srv.GracefulStop()

	rpcRegisterIP := config.Config.RpcRegisterIP
	rtc.RegisterRtcServiceServer(srv, rpc)
	if config.Config.RpcRegisterIP == "" {
		rpcRegisterIP, err = utils.GetLocalIP()
		if err != nil {
			log.Error("", "GetLocalIP failed ", err.Error())
		}
	}
	err = getcdv3.RegisterEtcd(rpc.etcdSchema, strings.Join(rpc.etcdAddr, ","), rpcRegisterIP, rpc.rpcPort, rpc.rpcRegisterName, 10)
	if err != nil {
		log.Error("", "register rpcChat to etcd failed ", err.Error())
		panic(utils.Wrap(err, "register chat module  rpc to etcd err"))
	}
	err = srv.Serve(listener)
	if err != nil {
		log.Error("", "rpc rpcChat failed ", err.Error())
		return
	}
	log.Info("", "rpc rpcChat init success")
}

func (rpc *rtcLiveKit) SignalGetRooms(context.Context, *pbRtc.SignalGetRoomsReq) (*pbRtc.SignalGetRoomsResp, error) {
	replay := pbRtc.SignalGetRoomsResp{}
	return &replay, nil
}

func (rpc *rtcLiveKit) SignalMessageAssemble(_ context.Context, req *pbRtc.SignalMessageAssembleReq) (*pbRtc.SignalMessageAssembleResp, error) {
	replay := pbRtc.SignalMessageAssembleResp{
		CommonResp: &pbRtc.CommonResp{},
		IsPass:     true,
		SignalResp: &pbRtc.SignalResp{},
		MsgData: &pbRtc.MsgData{
			//SenderPlatformID: constant.AndroidPlatformID,
			MsgFrom:    constant.UserMsgType,
			CreateTime: utils.GetCurrentTimestampByMill(),
			SendTime:   utils.GetCurrentTimestampByMill(),
			//SessionType:      constant.SingleChatType,
			ContentType:     constant.SignalingNotification,
			OfflinePushInfo: &pbRtc.OfflinePushInfo{Title: "offlinePush"},
		},
	}
	data, _ := proto.Marshal(req.SignalReq)
	replay.MsgData.Content = data
	invitationInfo := &pbRtc.InvitationInfo{}
	switch signalInfo := req.SignalReq.Payload.(type) {
	case *pbRtc.SignalReq_Invite:
		invitationInfo = signalInfo.Invite.Invitation
		roomId := invitationInfo.RoomID
		token, _ := rpc.getJoinToken(rpc.key, rpc.secret, roomId, signalInfo.Invite.OpUserID)
		replay.SignalResp.Payload = &pbRtc.SignalResp_Invite{
			Invite: &pbRtc.SignalInviteReply{
				Token:   token,
				RoomID:  roomId,
				LiveURL: rpc.LiveURL,
			},
		}
		replay.MsgData.ClientMsgID = utils.GetMsgID(signalInfo.Invite.OpUserID)
		replay.MsgData.SendID = invitationInfo.InviterUserID
		replay.MsgData.RecvID = invitationInfo.InviteeUserIDList[0]
		replay.MsgData.SenderPlatformID = invitationInfo.PlatformID
		replay.MsgData.SessionType = invitationInfo.SessionType
		log.Info(req.OperationID, "SignalReq_Invite :", replay.MsgData.String(), "recv:", signalInfo.Invite.String())
		log.Info(req.OperationID, "SignalReq_Invite", rpc.LiveURL)
		log.Info(req.OperationID, "SignalReq_Invite", token)
		break
	case *pbRtc.SignalReq_InviteInGroup:
		invitationInfo = signalInfo.InviteInGroup.Invitation
		roomId := invitationInfo.RoomID
		token, _ := rpc.getJoinToken(rpc.key, rpc.secret, roomId, signalInfo.InviteInGroup.OpUserID)
		replay.SignalResp.Payload = &pbRtc.SignalResp_Invite{
			Invite: &pbRtc.SignalInviteReply{
				Token:   token,
				RoomID:  roomId,
				LiveURL: rpc.LiveURL,
			},
		}
		replay.MsgData.ClientMsgID = utils.GetMsgID(invitationInfo.InviterUserID)
		replay.MsgData.SendID = invitationInfo.InviterUserID
		replay.MsgData.GroupID = invitationInfo.GroupID
		replay.MsgData.SenderPlatformID = invitationInfo.PlatformID
		replay.MsgData.SessionType = invitationInfo.SessionType
		log.Info(req.OperationID, "SignalReq_InviteInGroup :", replay.MsgData.String(), "recv:", signalInfo.InviteInGroup.String())
		break
	case *pbRtc.SignalReq_Cancel:
		invitationInfo = signalInfo.Cancel.Invitation
		replay.SignalResp.Payload = &pbRtc.SignalResp_Cancel{
			Cancel: &pbRtc.SignalCancelReply{},
		}
		replay.MsgData.ClientMsgID = utils.GetMsgID(invitationInfo.InviterUserID)
		replay.MsgData.SendID = signalInfo.Cancel.OpUserID
		replay.MsgData.RecvID = invitationInfo.InviteeUserIDList[0]
		replay.MsgData.SenderPlatformID = invitationInfo.PlatformID
		replay.MsgData.SessionType = invitationInfo.SessionType
		log.Info(req.OperationID, "SignalReq_Cancel :", replay.MsgData.String(), "recv:", signalInfo.Cancel.String())
		break
	case *pbRtc.SignalReq_Accept:
		invitationInfo = signalInfo.Accept.Invitation
		roomId := invitationInfo.RoomID
		token, _ := rpc.getJoinToken(rpc.key, rpc.secret, roomId, signalInfo.Accept.OpUserID)
		replay.SignalResp.Payload = &pbRtc.SignalResp_Accept{
			Accept: &pbRtc.SignalAcceptReply{
				Token:   token,
				RoomID:  roomId,
				LiveURL: config.Config.Rtc.LiveURL,
			},
		}
		replay.MsgData.ClientMsgID = utils.GetMsgID(signalInfo.Accept.OpUserID)
		replay.MsgData.SendID = signalInfo.Accept.OpUserID
		replay.MsgData.RecvID = invitationInfo.InviterUserID
		replay.MsgData.SenderPlatformID = signalInfo.Accept.OpUserPlatformID
		replay.MsgData.SessionType = signalInfo.Accept.Invitation.SessionType
		log.Info(req.OperationID, "SignalReq_Accept :", replay.MsgData.String(), "recv:", signalInfo.Accept.String())
		log.Info(req.OperationID, "SignalReq_Accept", token)
		break
	case *pbRtc.SignalReq_HungUp:
		log.Info(req.OperationID, "SignalReq_HungUp :", replay.MsgData.String(), "recv:", signalInfo.HungUp.String())
		break
	case *pbRtc.SignalReq_Reject:
		invitationInfo = signalInfo.Reject.Invitation
		replay.SignalResp.Payload = &pbRtc.SignalResp_Reject{
			Reject: &pbRtc.SignalRejectReply{},
		}
		replay.MsgData.ClientMsgID = utils.GetMsgID(signalInfo.Reject.OpUserID)
		replay.MsgData.SendID = signalInfo.Reject.OpUserID
		replay.MsgData.RecvID = invitationInfo.InviterUserID
		replay.MsgData.SenderPlatformID = signalInfo.Reject.OpUserPlatformID
		replay.MsgData.SessionType = signalInfo.Reject.Invitation.SessionType
		log.Info(req.OperationID, "SignalReq_Reject :", replay.MsgData.String(), "recv:", signalInfo.Reject.String())
		break
	default:

	}
	return &replay, nil
}

func (rpc *rtcLiveKit) getJoinToken(apiKey, apiSecret, room, identity string) (string, error) {
	at := auth.NewAccessToken(apiKey, apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}
	at.AddGrant(grant).
		SetIdentity(identity).
		SetValidFor(time.Hour)
	return at.ToJWT()
}

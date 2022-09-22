// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	rtcd "github.com/mattermost/rtcd/service"
	"github.com/mattermost/rtcd/service/rtc"

	"github.com/mattermost/mattermost-server/v6/model"
)

const (
	wsEventSignal           = "signal"
	wsEventUserConnected    = "user_connected"
	wsEventUserDisconnected = "user_disconnected"
	wsEventUserMuted        = "user_muted"
	wsEventUserUnmuted      = "user_unmuted"
	wsEventUserVoiceOn      = "user_voice_on"
	wsEventUserVoiceOff     = "user_voice_off"
	wsEventUserScreenOn     = "user_screen_on"
	wsEventUserScreenOff    = "user_screen_off"
	wsEventCallStart        = "call_start"
	wsEventCallEnd          = "call_end"
	wsEventUserRaiseHand    = "user_raise_hand"
	wsEventUserUnraiseHand  = "user_unraise_hand"
	wsEventUserReact        = "user_reaction"
	wsEventJoin             = "join"
	wsEventError            = "error"
	wsReconnectionTimeout   = 10 * time.Second
)

func (p *Plugin) handleClientMessageTypeScreen(us *session, msg clientMessage, handlerID string) error {
	data := map[string]string{}
	if msg.Type == clientMessageTypeScreenOn {
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			return err
		}
	}

	if err := p.kvSetAtomicChannelState(us.channelID, func(state *channelState) (*channelState, error) {
		if state == nil {
			return nil, fmt.Errorf("channel state is missing from store")
		}
		if state.Call == nil {
			return nil, fmt.Errorf("call state is missing from channel state")
		}

		if msg.Type == clientMessageTypeScreenOn {
			if state.Call.ScreenSharingID != "" {
				return nil, fmt.Errorf("cannot start screen sharing, someone else is sharing already: %q", state.Call.ScreenSharingID)
			}
			state.Call.ScreenSharingID = us.userID
			state.Call.ScreenStreamID = data["screenStreamID"]
		} else {
			if state.Call.ScreenSharingID != us.userID {
				return nil, fmt.Errorf("cannot stop screen sharing, someone else is sharing already: %q", state.Call.ScreenSharingID)
			}
			state.Call.ScreenSharingID = ""
			state.Call.ScreenStreamID = ""
		}

		return state, nil
	}); err != nil {
		return err
	}

	msgType := rtc.ScreenOnMessage
	wsMsgType := wsEventUserScreenOn
	if msg.Type == clientMessageTypeScreenOff {
		msgType = rtc.ScreenOffMessage
		wsMsgType = wsEventUserScreenOff
	}

	if handlerID != p.nodeID {
		if err := p.sendClusterMessage(clusterMessage{
			ConnID:        us.originalConnID,
			UserID:        us.userID,
			ChannelID:     us.channelID,
			SenderID:      p.nodeID,
			ClientMessage: msg,
		}, clusterMessageTypeUserState, handlerID); err != nil {
			return err
		}
	} else {
		rtcMsg := rtc.Message{
			SessionID: us.originalConnID,
			Type:      msgType,
			Data:      msg.Data,
		}

		if err := p.sendRTCMessage(rtcMsg, us.channelID); err != nil {
			return fmt.Errorf("failed to send RTC message: %w", err)
		}
	}

	p.API.PublishWebSocketEvent(wsMsgType, map[string]interface{}{
		"userID": us.userID,
	}, &model.WebsocketBroadcast{ChannelId: us.channelID, ReliableClusterSend: true})

	return nil
}

func (p *Plugin) handleClientMsg(us *session, msg clientMessage, handlerID string) {
	p.metrics.IncWebSocketEvent("in", msg.Type)
	switch msg.Type {
	case clientMessageTypeSDP:
		// if I am not the handler for this we relay the signaling message.
		if handlerID != p.nodeID {
			// need to relay signaling.
			if err := p.sendClusterMessage(clusterMessage{
				ConnID:        us.originalConnID,
				UserID:        us.userID,
				ChannelID:     us.channelID,
				SenderID:      p.nodeID,
				ClientMessage: msg,
			}, clusterMessageTypeSignaling, handlerID); err != nil {
				p.LogError(err.Error())
			}
		} else {
			rtcMsg := rtc.Message{
				SessionID: us.originalConnID,
				Type:      rtc.SDPMessage,
				Data:      msg.Data,
			}

			if err := p.sendRTCMessage(rtcMsg, us.channelID); err != nil {
				p.LogError(fmt.Errorf("failed to send RTC message: %w", err).Error())
			}
		}
	case clientMessageTypeICE:
		p.LogDebug("candidate!")
		if handlerID == p.nodeID {
			rtcMsg := rtc.Message{
				SessionID: us.originalConnID,
				Type:      rtc.ICEMessage,
				Data:      msg.Data,
			}

			if err := p.sendRTCMessage(rtcMsg, us.channelID); err != nil {
				p.LogError(fmt.Errorf("failed to send RTC message: %w", err).Error())
			}
		} else {
			// need to relay signaling.
			if err := p.sendClusterMessage(clusterMessage{
				ConnID:        us.originalConnID,
				UserID:        us.userID,
				ChannelID:     us.channelID,
				SenderID:      p.nodeID,
				ClientMessage: msg,
			}, clusterMessageTypeSignaling, handlerID); err != nil {
				p.LogError(err.Error())
			}
		}
	case clientMessageTypeMute, clientMessageTypeUnmute:
		if handlerID != p.nodeID {
			// need to relay track event.
			if err := p.sendClusterMessage(clusterMessage{
				ConnID:        us.originalConnID,
				UserID:        us.userID,
				ChannelID:     us.channelID,
				SenderID:      p.nodeID,
				ClientMessage: msg,
			}, clusterMessageTypeUserState, handlerID); err != nil {
				p.LogError(err.Error())
			}
		} else {
			msgType := rtc.UnmuteMessage
			if msg.Type == clientMessageTypeMute {
				msgType = rtc.MuteMessage
			}

			rtcMsg := rtc.Message{
				SessionID: us.originalConnID,
				Type:      msgType,
				Data:      msg.Data,
			}

			if err := p.sendRTCMessage(rtcMsg, us.channelID); err != nil {
				p.LogError(fmt.Errorf("failed to send RTC message: %w", err).Error())
			}
		}

		if err := p.kvSetAtomicChannelState(us.channelID, func(state *channelState) (*channelState, error) {
			if state == nil {
				return nil, fmt.Errorf("channel state is missing from store")
			}
			if state.Call == nil {
				return nil, fmt.Errorf("call state is missing from channel state")
			}
			if uState := state.Call.Users[us.userID]; uState != nil {
				uState.Unmuted = msg.Type == clientMessageTypeUnmute
			}

			return state, nil
		}); err != nil {
			p.LogError(err.Error())
		}

		evType := wsEventUserUnmuted
		if msg.Type == clientMessageTypeMute {
			evType = wsEventUserMuted
		}
		p.API.PublishWebSocketEvent(evType, map[string]interface{}{
			"userID": us.userID,
		}, &model.WebsocketBroadcast{ChannelId: us.channelID, ReliableClusterSend: true})
	case clientMessageTypeVoiceOn, clientMessageTypeVoiceOff:
		evType := wsEventUserVoiceOff
		if msg.Type == clientMessageTypeVoiceOn {
			evType = wsEventUserVoiceOn
		}
		p.API.PublishWebSocketEvent(evType, map[string]interface{}{
			"userID": us.userID,
		}, &model.WebsocketBroadcast{ChannelId: us.channelID, ReliableClusterSend: true})
	case clientMessageTypeScreenOn, clientMessageTypeScreenOff:
		if err := p.handleClientMessageTypeScreen(us, msg, handlerID); err != nil {
			p.LogError(err.Error())
		}
	case clientMessageTypeRaiseHand, clientMessageTypeUnraiseHand:
		evType := wsEventUserUnraiseHand
		if msg.Type == clientMessageTypeRaiseHand {
			evType = wsEventUserRaiseHand
		}

		var ts int64
		if msg.Type == clientMessageTypeRaiseHand {
			ts = time.Now().UnixMilli()
		}

		if err := p.kvSetAtomicChannelState(us.channelID, func(state *channelState) (*channelState, error) {
			if state == nil {
				return nil, fmt.Errorf("channel state is missing from store")
			}
			if state.Call == nil {
				return nil, fmt.Errorf("call state is missing from channel state")
			}
			if uState := state.Call.Users[us.userID]; uState != nil {
				uState.RaisedHand = ts
			}

			return state, nil
		}); err != nil {
			p.LogError(err.Error())
		}

		p.API.PublishWebSocketEvent(evType, map[string]interface{}{
			"userID":      us.userID,
			"raised_hand": ts,
		}, &model.WebsocketBroadcast{ChannelId: us.channelID, ReliableClusterSend: true})
	case clientMessageTypeReaction:
		evType := wsEventUserReact

		var data struct {
			Emoji string `json:"emoji"`
		}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			p.LogError(err.Error())
		}

		p.API.PublishWebSocketEvent(evType, map[string]interface{}{
			"userID":    us.userID,
			"emoji":     data.Emoji,
			"timestamp": time.Now().UnixMilli(),
		}, &model.WebsocketBroadcast{ChannelId: us.channelID})
	default:
		p.LogError("invalid client message", "type", msg.Type)
		return
	}
}

func (p *Plugin) OnWebSocketDisconnect(connID, userID string) {
	if userID == "" {
		return
	}

	p.mut.RLock()
	us := p.sessions[connID]
	p.mut.RUnlock()
	if us != nil {
		if atomic.CompareAndSwapInt32(&us.wsClosed, 0, 1) {
			p.LogDebug("closing ws channel for session", "userID", userID, "connID", connID, "channelID", us.channelID)
			close(us.wsCloseCh)
		} else {
			p.LogError("ws channel already closed", "userID", userID, "connID", connID, "channelID", us.channelID)
		}
	}
}

func (p *Plugin) wsReader(us *session, handlerID string) {
	for {
		select {
		case msg, ok := <-us.wsMsgCh:
			if !ok {
				return
			}
			p.handleClientMsg(us, msg, handlerID)
		case <-us.wsReconnectCh:
			return
		case <-us.leaveCh:
			return
		case <-us.wsCloseCh:
			return
		case <-us.rtcCloseCh:
			return
		}
	}
}

func (p *Plugin) sendRTCMessage(msg rtc.Message, channelID string) error {
	if p.rtcdManager != nil {
		cm := rtcd.ClientMessage{
			Type: rtcd.ClientMessageRTC,
			Data: msg,
		}
		return p.rtcdManager.Send(cm, channelID)
	}

	return p.rtcServer.Send(msg)
}

func (p *Plugin) wsWriter() {
	for {
		select {
		case msg, ok := <-p.rtcServer.ReceiveCh():
			if !ok {
				return
			}
			p.mut.RLock()
			us := p.sessions[msg.SessionID]
			p.mut.RUnlock()
			if us == nil {
				p.LogError("session should not be nil")
				continue
			}
			p.metrics.IncWebSocketEvent("out", "signal")
			p.API.PublishWebSocketEvent(wsEventSignal, map[string]interface{}{
				"data":   string(msg.Data),
				"connID": msg.SessionID,
			}, &model.WebsocketBroadcast{UserId: us.userID, ReliableClusterSend: true})
		case <-p.stopCh:
			return
		}
	}
}

func (p *Plugin) handleLeave(us *session, userID, connID, channelID string) error {
	p.LogDebug("handleLeave", "userID", userID, "connID", connID, "channelID", channelID)

	select {
	case <-us.wsReconnectCh:
		p.LogDebug("reconnected, returning", "userID", userID, "connID", connID, "channelID", channelID)
		return nil
	case <-us.leaveCh:
		p.LogDebug("user left call", "userID", userID, "connID", connID, "channelID", us.channelID)
	case <-us.rtcCloseCh:
		p.LogDebug("rtc connection was closed", "userID", userID, "connID", connID, "channelID", us.channelID)
		return nil
	case <-time.After(wsReconnectionTimeout):
		p.LogDebug("timeout waiting for reconnection", "userID", userID, "connID", connID, "channelID", channelID)
	}

	state, err := p.kvGetChannelState(channelID)
	if err != nil {
		return err
	} else if state != nil && state.Call != nil && state.Call.ScreenSharingID == userID {
		p.API.PublishWebSocketEvent(wsEventUserScreenOff, map[string]interface{}{}, &model.WebsocketBroadcast{ChannelId: channelID, ReliableClusterSend: true})
	}

	handlerID, err := p.getHandlerID()
	if err != nil {
		p.LogError(err.Error())
	}
	if handlerID == "" && state != nil {
		handlerID = state.NodeID
	}

	if err := p.closeRTCSession(userID, us.originalConnID, channelID, handlerID); err != nil {
		p.LogError(err.Error())
	}

	if err := p.removeSession(us); err != nil {
		p.LogError(err.Error())
	}

	if state != nil && state.Call != nil {
		p.track(evCallUserLeft, map[string]interface{}{
			"ParticipantID": userID,
			"ChannelID":     channelID,
			"CallID":        state.Call.ID,
		})
	}

	return nil
}

func (p *Plugin) handleJoin(userID, connID, channelID, title string) error {
	p.LogDebug("handleJoin", "userID", userID, "connID", connID, "channelID", channelID)

	if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionCreatePost) {
		return fmt.Errorf("forbidden")
	}
	channel, appErr := p.API.GetChannel(channelID)
	if appErr != nil {
		return appErr
	}
	if channel.DeleteAt > 0 {
		return fmt.Errorf("cannot join call in archived channel")
	}

	state, err := p.addUserSession(userID, connID, channel)
	if err != nil {
		return fmt.Errorf("failed to add user session: %w", err)
	} else if state.Call == nil {
		return fmt.Errorf("state.Call should not be nil")
	} else if len(state.Call.Users) == 1 {
		p.track(evCallStarted, map[string]interface{}{
			"ParticipantID": userID,
			"CallID":        state.Call.ID,
			"ChannelID":     channelID,
			"ChannelType":   channel.Type,
		})

		// new call has started
		threadID, err := p.startNewCallThread(userID, channelID, state.Call.StartAt, title)
		if err != nil {
			p.LogError(err.Error())
		}

		// TODO: send all the info attached to a call.
		p.API.PublishWebSocketEvent(wsEventCallStart, map[string]interface{}{
			"channelID": channelID,
			"start_at":  state.Call.StartAt,
			"thread_id": threadID,
			"owner_id":  state.Call.OwnerID,
		}, &model.WebsocketBroadcast{ChannelId: channelID, ReliableClusterSend: true})
	}

	handlerID, err := p.getHandlerID()
	if err != nil {
		p.LogError(err.Error())
	}
	if handlerID == "" {
		handlerID = state.NodeID
	}
	p.LogDebug("got handlerID", "handlerID", handlerID)

	us := newUserSession(userID, channelID, connID, p.rtcdManager == nil && handlerID == p.nodeID)
	p.mut.Lock()
	p.sessions[connID] = us
	p.mut.Unlock()
	defer func() {
		if err := p.handleLeave(us, userID, connID, channelID); err != nil {
			p.LogError(err.Error())
		}
	}()

	if p.rtcdManager != nil {
		msg := rtcd.ClientMessage{
			Type: rtcd.ClientMessageJoin,
			Data: map[string]string{
				"callID":    channelID,
				"userID":    userID,
				"sessionID": connID,
			},
		}
		if err := p.rtcdManager.Send(msg, channelID); err != nil {
			return fmt.Errorf("failed to send client join message: %w", err)
		}
	} else {
		if handlerID == p.nodeID {
			cfg := rtc.SessionConfig{
				GroupID:   "default",
				CallID:    channelID,
				UserID:    userID,
				SessionID: connID,
			}
			p.LogDebug("initializing RTC session", "userID", userID, "connID", connID, "channelID", channelID)
			if err = p.rtcServer.InitSession(cfg, func() error {
				if atomic.CompareAndSwapInt32(&us.rtcClosed, 0, 1) {
					close(us.rtcCloseCh)
					return p.removeSession(us)
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to init session: %w", err)
			}
		} else {
			if err := p.sendClusterMessage(clusterMessage{
				ConnID:    connID,
				UserID:    userID,
				ChannelID: channelID,
				SenderID:  p.nodeID,
			}, clusterMessageTypeConnect, handlerID); err != nil {
				return fmt.Errorf("failed to send connect message: %w", err)
			}
		}
	}

	// send successful join response
	p.metrics.IncWebSocketEvent("out", "join")
	p.API.PublishWebSocketEvent(wsEventJoin, map[string]interface{}{
		"connID": connID,
	}, &model.WebsocketBroadcast{UserId: userID, ReliableClusterSend: true})
	p.metrics.IncWebSocketEvent("out", "user_connected")
	p.API.PublishWebSocketEvent(wsEventUserConnected, map[string]interface{}{
		"userID": userID,
	}, &model.WebsocketBroadcast{ChannelId: channelID, ReliableClusterSend: true})
	p.metrics.IncWebSocketConn(channelID)
	defer p.metrics.DecWebSocketConn(channelID)
	p.track(evCallUserJoined, map[string]interface{}{
		"ParticipantID": userID,
		"ChannelID":     channelID,
		"CallID":        state.Call.ID,
	})

	p.wsReader(us, handlerID)

	return nil
}

func (p *Plugin) handleReconnect(userID, connID, channelID, originalConnID, prevConnID string) error {
	p.LogDebug("handleReconnect", "userID", userID, "connID", connID, "channelID", channelID,
		"originalConnID", originalConnID, "prevConnID", prevConnID)

	if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionCreatePost) {
		return fmt.Errorf("forbidden")
	}

	state, err := p.kvGetChannelState(channelID)
	if err != nil {
		return err
	} else if state == nil || state.Call == nil {
		return fmt.Errorf("call state not found")
	} else if _, ok := state.Call.Sessions[originalConnID]; !ok {
		return fmt.Errorf("session not found in call state")
	}

	var rtc bool
	p.mut.Lock()
	us := p.sessions[connID]
	if us != nil {
		rtc = us.rtc
		if atomic.CompareAndSwapInt32(&us.wsReconnected, 0, 1) {
			p.LogDebug("closing reconnectCh", "userID", userID, "connID", connID, "channelID", channelID,
				"originalConnID", originalConnID)
			close(us.wsReconnectCh)
		} else {
			p.mut.Unlock()
			return fmt.Errorf("session already reconnected")
		}
	} else {
		p.LogDebug("session not found", "connID", connID)
	}

	us = newUserSession(userID, channelID, connID, rtc)
	us.originalConnID = originalConnID
	p.sessions[connID] = us
	p.mut.Unlock()

	if err := p.sendClusterMessage(clusterMessage{
		ConnID:   prevConnID,
		UserID:   userID,
		SenderID: p.nodeID,
	}, clusterMessageTypeReconnect, ""); err != nil {
		p.LogError(err.Error())
	}

	if p.rtcdManager != nil {
		msg := rtcd.ClientMessage{
			Type: rtcd.ClientMessageReconnect,
			Data: map[string]string{
				"sessionID": originalConnID,
			},
		}
		if err := p.rtcdManager.Send(msg, channelID); err != nil {
			return fmt.Errorf("failed to send client reconnect message: %w", err)
		}
	}

	handlerID, err := p.getHandlerID()
	if err != nil {
		p.LogError(err.Error())
	}
	if handlerID == "" && state != nil {
		handlerID = state.NodeID
	}

	p.wsReader(us, handlerID)

	if err := p.handleLeave(us, userID, connID, channelID); err != nil {
		p.LogError(err.Error())
	}

	return nil
}

func (p *Plugin) WebSocketMessageHasBeenPosted(connID, userID string, req *model.WebSocketRequest) {
	var msg clientMessage
	msg.Type = strings.TrimPrefix(req.Action, wsActionPrefix)

	p.mut.RLock()
	us := p.sessions[connID]
	p.mut.RUnlock()

	if msg.Type != clientMessageTypeJoin &&
		msg.Type != clientMessageTypeLeave &&
		msg.Type != clientMessageTypeReconnect && us == nil {
		return
	}

	if us != nil && !us.limiter.Allow() {
		p.LogError("message was dropped by rate limiter", "msgType", msg.Type, "userID", us.userID, "connID", us.connID)
		return
	}

	switch msg.Type {
	case clientMessageTypeJoin:
		channelID, ok := req.Data["channelID"].(string)
		if !ok {
			p.LogError("missing channelID")
			return
		}

		// Title is optional, so if it's not present,
		// it will be an empty string.
		title, _ := req.Data["title"].(string)
		go func() {
			if err := p.handleJoin(userID, connID, channelID, title); err != nil {
				p.LogError(err.Error(), "userID", userID, "connID", connID, "channelID", channelID)
				p.metrics.IncWebSocketEvent("out", "error")
				p.API.PublishWebSocketEvent(wsEventError, map[string]interface{}{
					"data":   err.Error(),
					"connID": connID,
				}, &model.WebsocketBroadcast{UserId: userID, ReliableClusterSend: true})
				return
			}
		}()
		return
	case clientMessageTypeReconnect:
		p.metrics.IncWebSocketEvent("in", "reconnect")

		channelID, _ := req.Data["channelID"].(string)
		if channelID == "" {
			p.LogError("missing channelID")
			return
		}
		originalConnID, _ := req.Data["originalConnID"].(string)
		if originalConnID == "" {
			p.LogError("missing originalConnID")
			return
		}
		prevConnID, _ := req.Data["prevConnID"].(string)
		if prevConnID == "" {
			p.LogError("missing prevConnID")
			return
		}

		go func() {
			if err := p.handleReconnect(userID, connID, channelID, originalConnID, prevConnID); err != nil {
				p.LogError(err.Error(), "userID", userID, "connID", connID,
					"originalConnID", originalConnID, "prevConnID", prevConnID, "channelID", channelID)
			}
		}()
		return
	case clientMessageTypeLeave:
		p.metrics.IncWebSocketEvent("in", "leave")
		p.LogDebug("leave message", "userID", userID, "connID", connID)

		if us != nil && atomic.CompareAndSwapInt32(&us.left, 0, 1) {
			close(us.leaveCh)
		}

		if err := p.sendClusterMessage(clusterMessage{
			ConnID:   connID,
			UserID:   userID,
			SenderID: p.nodeID,
		}, clusterMessageTypeLeave, ""); err != nil {
			p.LogError(err.Error())
		}

		return
	case clientMessageTypeSDP:
		msgData, ok := req.Data["data"].([]byte)
		if !ok {
			p.LogError("invalid or missing sdp data")
			return
		}
		data, err := unpackSDPData(msgData)
		if err != nil {
			p.LogError(err.Error())
			return
		}
		msg.Data = data
	case clientMessageTypeICE, clientMessageTypeScreenOn:
		msgData, ok := req.Data["data"].(string)
		if !ok {
			p.LogError("invalid or missing data")
			return
		}
		msg.Data = []byte(msgData)
	case clientMessageTypeReaction:
		msgData, ok := req.Data["data"].(string)
		if !ok {
			p.LogError("invalid or missing data")
			return
		}
		msg.Data = []byte(msgData)

	}

	select {
	case us.wsMsgCh <- msg:
	default:
		p.LogError("chan is full, dropping ws msg", "type", msg.Type)
		return
	}
}

func (p *Plugin) closeRTCSession(userID, connID, channelID, handlerID string) error {
	p.LogDebug("closeRTCSession", "userID", userID, "connID", connID, "channelID", channelID)
	if p.rtcServer != nil {
		if handlerID == p.nodeID {
			if err := p.rtcServer.CloseSession(connID); err != nil {
				return err
			}
		} else {
			if err := p.sendClusterMessage(clusterMessage{
				ConnID:    connID,
				UserID:    userID,
				ChannelID: channelID,
				SenderID:  p.nodeID,
			}, clusterMessageTypeDisconnect, handlerID); err != nil {
				return err
			}
		}
	} else if p.rtcdManager != nil {
		msg := rtcd.ClientMessage{
			Type: rtcd.ClientMessageLeave,
			Data: map[string]string{
				"sessionID": connID,
			},
		}
		if err := p.rtcdManager.Send(msg, channelID); err != nil {
			return fmt.Errorf("failed to send client message: %w", err)
		}
	}

	return nil
}

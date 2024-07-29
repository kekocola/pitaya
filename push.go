// Copyright (c) TFG Co. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package pitaya

import (
	"strconv"

	"github.com/topfreegames/pitaya/v2/cluster"
	"github.com/topfreegames/pitaya/v2/conn/message"
	"github.com/topfreegames/pitaya/v2/constants"
	"github.com/topfreegames/pitaya/v2/logger"
	"github.com/topfreegames/pitaya/v2/protos"
	"github.com/topfreegames/pitaya/v2/util"
)

// SendPushToUsers sends a message to the given list of users
func (app *App) SendPushToUsers(route string, v interface{}, uids []string, frontendType string) ([]string, error) {
	if !message.IsRouteValid(route) {
		logger.Log.Errorf("route:%s undefined", route)
		return uids, constants.ErrRouteUndefined
	}

	data, err := util.SerializeOrRaw(app.serializer, v)
	if err != nil {
		return uids, err
	}

	if !app.server.Frontend && frontendType == "" {
		return uids, constants.ErrFrontendTypeNotSpecified
	}

	var notPushedUids []string
	var multiPushUsers []string

	logger.Log.Debugf("Type=PushToUsers Route=%s, Data=%+v, SvType=%s, #Users=%d", route, v, frontendType, len(uids))

	for _, uid := range uids {
		if s := app.sessionPool.GetSessionByUID(uid); s != nil && app.server.Type == frontendType {
			if err := s.Push(route, data); err != nil {
				notPushedUids = append(notPushedUids, uid)
				logger.Log.Errorf("Session push message error, Route=%s, ID=%d, UID=%s, Error=%s",
					route, s.ID(), s.UID(), err.Error())
			}
		} else {
			id, err := strconv.ParseUint(uid, 10, 64)
			if err != nil {
				logger.Log.Errorf("Invalid UID, UID=%s", uid)
				notPushedUids = append(notPushedUids, uid)
				continue
			}
			// only Multi Push online users
			if app.IsUserOnline(id) {
				multiPushUsers = append(multiPushUsers, uid)
			} else {
				notPushedUids = append(notPushedUids, uid)
			}
		}
	}

	if len(multiPushUsers) > 0 {
		err = app.rpcClient.PushToUsers(multiPushUsers, frontendType, &protos.MultiPush{Route: route, Data: data})
		if err != nil {
			notPushedUids = append(notPushedUids, multiPushUsers...)
			logger.Log.Errorf("RPCClient send message error, Route=%s, UIDs=%v, SvType=%s, Error=%s", route, multiPushUsers, frontendType, err.Error())
		}
	}

	if len(notPushedUids) != 0 {
		return notPushedUids, constants.ErrPushingToUsers
	}

	return nil, nil
}

// PushMsg pushes message to an user
func (app *App) PushMsg(uid uint64, route string, v interface{}, frontendType string) error {
	if !message.IsRouteValid(route) {
		logger.Log.Errorf("route:%s undefined", route)
		return constants.ErrRouteUndefined
	}

	data, err := util.SerializeOrRaw(app.serializer, v)
	if err != nil {
		return err
	}

	if !app.server.Frontend && frontendType == "" {
		return constants.ErrFrontendTypeNotSpecified
	}

	strUid := strconv.FormatUint(uid, 10)
	if app.server.Frontend && app.server.Type == frontendType {
		if s := app.sessionPool.GetSessionByUID(strUid); s != nil {
			if err := s.Push(route, data); err != nil {
				logger.Log.Errorf("Session push message error, Route=%s, ID=%d, UID=%d, Error=%s",
					route, s.ID(), uid, err.Error())
				return err
			} else {
				return nil
			}
		}
	}

	push := &protos.Push{
		Route: route,
		Uid:   strUid,
		Data:  data,
	}
	if err = app.rpcClient.SendPush(strUid, &cluster.Server{Type: frontendType}, push); err != nil {
		logger.Log.Errorf("RPCClient send message error, Route=%s, UID=%s, SvType=%s, Error=%s", route, uid, frontendType, err.Error())
		return err
	}

	return nil
}

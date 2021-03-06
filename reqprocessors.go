/*
 * Copyright 2011 Nan Deng
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"fmt"
	"github.com/uniqush/log"
	. "github.com/uniqush/pushdb"
	"regexp"
)

type RequestProcessor interface {
	SetLogger(logger *uniqushlog.Logger)
	Process(req *Request)
}

type ActionPrinter struct {
	logger *uniqushlog.Logger
}

func NewActionPrinter(logger *uniqushlog.Logger) RequestProcessor {
	a := new(ActionPrinter)
	a.logger = logger
	return a
}

func (p *ActionPrinter) SetLogger(logger *uniqushlog.Logger) {
	p.logger = logger
}

func (p *ActionPrinter) Process(r *Request) {
	p.logger.Debugf("Action: %d-%s, id: %s\n", r.Action, r.ActionName(), r.ID)
	r.Finish()
	return
}

type logSetter struct {
	logger *uniqushlog.Logger
}

func (l *logSetter) SetLogger(logger *uniqushlog.Logger) {
	l.logger = logger
}

type databaseSetter struct {
	dbfront PushDatabase
}

func (d *databaseSetter) SetDatabase(dbfront PushDatabase) {
	d.dbfront = dbfront
}

type requestValidator struct {
	validSubscriberPattern *regexp.Regexp
	validServicePattern    *regexp.Regexp
}

func (r *requestValidator) InitValidator() {
	var err error
	r.validServicePattern, err = regexp.Compile("^[a-zA-z\\.0-9-_]+$")
	if err != nil {
		r.validServicePattern = nil
	}
	r.validSubscriberPattern, err = regexp.Compile("^[a-zA-z\\.0-9-_]+$")
	if err != nil {
		r.validSubscriberPattern = nil
	}
}

func (r *requestValidator) Validate(req *Request) error {
	for _, sub := range req.Subscribers {
		if !r.validSubscriberPattern.MatchString(sub) {
			return fmt.Errorf("invalid subscriber name: %s. Accept charaters: a-z, A-Z, 0-9, -, _ or .", sub)
		}
	}
	if !r.validServicePattern.MatchString(req.Service) {
		return fmt.Errorf("invalid service name: %s. Accept charaters: a-z, A-Z, 0-9, -, _ or .", req.Service)
	}
	return nil
}

type AddPushServiceProviderProcessor struct {
	logSetter
	databaseSetter
	requestValidator
}

func NewAddPushServiceProviderProcessor(logger *uniqushlog.Logger, dbfront PushDatabase) RequestProcessor {
	ret := new(AddPushServiceProviderProcessor)
	ret.SetLogger(logger)
	ret.SetDatabase(dbfront)
	ret.InitValidator()

	return ret
}

func (p *AddPushServiceProviderProcessor) Process(req *Request) {
	defer req.Finish()

	err := p.Validate(req)
	if err != nil {
		p.logger.Errorf("[AddPushServiceRequestFail] RequestId=%v %v", req.ID, err)
		req.Respond(fmt.Errorf("[AddPushServiceRequestFail] RequestId=%v %v", req.ID, err))
		return
	}
	err = p.dbfront.AddPushServiceProviderToService(req.Service, req.PushServiceProvider)
	if err != nil {
		p.logger.Errorf("[AddPushServiceRequestFail] RequestId=%v DatabaseError %v", req.ID, err)
		req.Respond(fmt.Errorf("[AddPushServiceRequestFail] RequestId=%v DatabaseError %v", req.ID, err))
		return
	}
	p.logger.Infof("[AddPushServiceRequest] RequestId=%v Success PushServiceProviderID=%s",
		req.ID, req.PushServiceProvider.Name())
	e := fmt.Errorf("PushServiceProvider=%v Success!", req.PushServiceProvider.Name())
	req.Respond(e)
	return
}

type RemovePushServiceProviderProcessor struct {
	logSetter
	databaseSetter
	requestValidator
}

func NewRemovePushServiceProviderProcessor(logger *uniqushlog.Logger,
	dbfront PushDatabase) RequestProcessor {
	ret := new(RemovePushServiceProviderProcessor)
	ret.SetLogger(logger)
	ret.SetDatabase(dbfront)
	ret.InitValidator()

	return ret
}

func (p *RemovePushServiceProviderProcessor) Process(req *Request) {
	defer req.Finish()

	err0 := p.Validate(req)
	if err0 != nil {
		p.logger.Errorf("[RemovePushServiceRequestFail] RequestId=%v %v", req.ID, err0)
		req.Respond(fmt.Errorf("[RemovePushServiceRequestFail] RequestId=%v %v", req.ID, err0))
		return
	}

	err := p.dbfront.RemovePushServiceProviderFromService(req.Service, req.PushServiceProvider)
	if err != nil {
		p.logger.Errorf("[RemovePushServiceRequestFail] RequestId=%v DatabaseError %v", req.ID, err)
		req.Respond(fmt.Errorf("[RemovePushServiceRequestFail] RequestId=%v DatabaseError %v", req.ID, err))
		return
	}
	p.logger.Infof("[RemovePushServiceRequest] Success PushServiceProviderID=%s", req.PushServiceProvider.Name())
	return
}

type SubscribeProcessor struct {
	logSetter
	databaseSetter
	requestValidator
}

func NewSubscribeProcessor(logger *uniqushlog.Logger, dbfront PushDatabase) RequestProcessor {
	ret := new(SubscribeProcessor)
	ret.SetLogger(logger)
	ret.SetDatabase(dbfront)
	ret.InitValidator()

	return ret
}

func (p *SubscribeProcessor) Process(req *Request) {
	defer req.Finish()
	if len(req.Subscribers) == 0 {
		return
	}

	err0 := p.Validate(req)
	if err0 != nil {
		p.logger.Errorf("[SubscribeRequestFail] RequestId=%v %v", req.ID, err0)
		req.Respond(fmt.Errorf("[SubscribeRequestFail] RequestId=%v %v", req.ID, err0))
		return
	}

	psp, err := p.dbfront.AddDeliveryPointToService(req.Service,
		req.Subscribers[0],
		req.DeliveryPoint)
	if err != nil || psp == nil {
		p.logger.Errorf("[SubscribeRequestFail] RequestId=%v DatabaseError %v", req.ID, err)
		req.Respond(fmt.Errorf("[SubscribeRequestFail] RequestId=%v DatabaseError %v", req.ID, err))
		return
	}
	dpname := req.DeliveryPoint.Name()
	pspname := psp.Name()
	p.logger.Infof("[SubscribeRequest] RequestId=%v Success DeliveryPoint=%s PushServiceProvider=%s",
		req.ID, dpname, pspname)
	e := fmt.Errorf("DeliveryPoint=%v Success!", dpname)
	req.Respond(e)
	return
}

type UnsubscribeProcessor struct {
	logSetter
	databaseSetter
	requestValidator
}

func NewUnsubscribeProcessor(logger *uniqushlog.Logger, dbfront PushDatabase) RequestProcessor {
	ret := new(UnsubscribeProcessor)
	ret.SetLogger(logger)
	ret.SetDatabase(dbfront)
	ret.InitValidator()

	return ret
}

func (p *UnsubscribeProcessor) Process(req *Request) {
	defer req.Finish()
	if len(req.Subscribers) == 0 || req.DeliveryPoint == nil {
		p.logger.Errorf("[UnSubscribeRequestFail] RequestId=%v Nil Pointer", req.ID)
		req.Respond(fmt.Errorf("[UnSubscribeRequestFail] RequestId=%v Nil Pointer", req.ID))
		return

	}
	err0 := p.Validate(req)
	if err0 != nil {
		p.logger.Errorf("[UnSubscribeRequestFail] RequestId=%v %v", req.ID, err0)
		req.Respond(fmt.Errorf("[UnSubscribeRequestFail] RequestId=%v %v", req.ID, err0))
		return
	}

	err := p.dbfront.RemoveDeliveryPointFromService(req.Service,
		req.Subscribers[0],
		req.DeliveryPoint)
	if err != nil {
		p.logger.Errorf("[UnSubscribeRequestFail] RequestId=%v DatabaseError %v", req.ID, err)
		req.Respond(fmt.Errorf("[UnSubscribeRequestFail] RequestId=%v DatabaseError %v", req.ID, err))
		return
	}
	p.logger.Infof("[UnsubscribeRequest] RequestId=%v Success DeliveryPoint=%s",
		req.ID, req.DeliveryPoint.Name())
	return
}

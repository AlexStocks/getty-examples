/******************************************************
# DESC    : echo package handler
# AUTHOR  : Alex Stocks
# LICENCE : Apache License 2.0
# EMAIL   : alexstocks@foxmail.com
# MOD     : 2016-09-04 13:08
# FILE    : handler.go
******************************************************/

package main

import (
	"errors"
	"sync"
	"time"
)

import (
	"github.com/AlexStocks/getty"
	log "github.com/AlexStocks/log4go"
)

var (
	errTooManySessions = errors.New("Too many echo sessions!")
)

type PackageHandler interface {
	Handle(getty.Session, *EchoPackage) error
}

////////////////////////////////////////////
// message handler
////////////////////////////////////////////

type MessageHandler struct{}

func (h *MessageHandler) Handle(session getty.Session, pkg *EchoPackage) error {
	log.Debug("get echo package{%s}", pkg)
	// write echo message handle logic here.
	return session.WritePkg(pkg, conf.GettySessionParam.waitTimeout)
}

////////////////////////////////////////////
// EchoMessageHandler
////////////////////////////////////////////

type clientEchoSession struct {
	session getty.Session
	reqNum  int32
}

type EchoMessageHandler struct {
	handlers map[uint32]PackageHandler

	rwlock     sync.RWMutex
	sessionMap map[getty.Session]*clientEchoSession
}

func newEchoMessageHandler() *EchoMessageHandler {
	handlers := make(map[uint32]PackageHandler)
	handlers[echoCmd] = &MessageHandler{}

	return &EchoMessageHandler{sessionMap: make(map[getty.Session]*clientEchoSession), handlers: handlers}
}

func (h *EchoMessageHandler) OnOpen(session getty.Session) error {
	var (
		err error
	)

	h.rwlock.RLock()
	if conf.SessionNumber <= len(h.sessionMap) {
		err = errTooManySessions
	}
	h.rwlock.RUnlock()
	if err != nil {
		return err
	}

	log.Info("got session:%s", session.Stat())
	h.rwlock.Lock()
	h.sessionMap[session] = &clientEchoSession{session: session}
	h.rwlock.Unlock()
	return nil
}

func (h *EchoMessageHandler) OnError(session getty.Session, err error) {
	log.Info("session{%s} got error{%v}, will be closed.", session.Stat(), err)
	h.rwlock.Lock()
	delete(h.sessionMap, session)
	h.rwlock.Unlock()
}

func (h *EchoMessageHandler) OnClose(session getty.Session) {
	log.Info("session{%s} is closing......", session.Stat())
	h.rwlock.Lock()
	delete(h.sessionMap, session)
	h.rwlock.Unlock()
}

func (h *EchoMessageHandler) OnMessage(session getty.Session, pkg interface{}) {
	p, ok := pkg.(*EchoPackage)
	if !ok {
		log.Error("illegal packge{%#v}, %s", pkg, string(pkg.([]byte)))
		return
	}

	handler, ok := h.handlers[p.H.Command]
	if !ok {
		log.Error("illegal command{%d}", p.H.Command)
		return
	}
	err := handler.Handle(session, p)
	if err != nil {
		h.rwlock.Lock()
		if _, ok := h.sessionMap[session]; ok {
			h.sessionMap[session].reqNum++
		}
		h.rwlock.Unlock()
	}
}

func (h *EchoMessageHandler) OnCron(session getty.Session) {
	var (
		flag   bool
		active time.Time
	)

	h.rwlock.RLock()
	if _, ok := h.sessionMap[session]; ok {
		active = session.GetActive()
		if conf.sessionTimeout.Nanoseconds() < time.Since(active).Nanoseconds() {
			flag = true
			log.Warn("session{%s} timeout{%s}, reqNum{%d}",
				session.Stat(), time.Since(active).String(), h.sessionMap[session].reqNum)
		}
	}
	h.rwlock.RUnlock()

	if flag {
		h.rwlock.Lock()
		delete(h.sessionMap, session)
		h.rwlock.Unlock()
		session.Close()
	}
}

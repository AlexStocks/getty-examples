// Copyright (C) 2010, Kyle Lemons <kyle@kylelemons.net>.  All rights reserved.

package log4go

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// This log writer sends output to a socket
type SocketLogWriter struct {
	LogCloser
	caller bool
	rec    chan LogRecord
	sync.Once
}

// This is the SocketLogWriter's output method
func (w *SocketLogWriter) LogWrite(rec *LogRecord) {
	defer func() {
		if e := recover(); e != nil {
			//js, err := json.Marshal(rec)
			//if err != nil {
			//	fmt.Printf("json error: %s", err)
			//	return
			//}
			//fmt.Printf("sock log channel has been closed. " + string(js) + "\n")
			// recJson, _ := json.Marshal(rec)
			fmt.Printf("sock log channel has been closed. " + String(rec.JSON()) + "\n")
		}
	}()

	recBufLen := len(w.rec)
	if recBufLen < SocketLogBufferLength {
		w.rec <- *rec
	} else {
		fmt.Fprintf(os.Stderr,
			"recBufLen:%d, LogBufferLength:%d, logRecord:%+v\n",
			recBufLen, SocketLogBufferLength, rec)
	}
}

func (w *SocketLogWriter) Close() {
	w.Once.Do(func() {
		w.WaitClosed(w.rec)
		close(w.rec)
	})
}

// This func shows whether output filename/function/lineno info in log
func (w *SocketLogWriter) GetCallerFlag() bool { return w.caller }

// Set whether output the filename/function name/line number info or not.
// Must be called before the first log message is written.
func (w *SocketLogWriter) SetCallerFlag(flag bool) *SocketLogWriter {
	w.caller = flag
	return w
}

func NewSocketLogWriter(proto, hostport string) *SocketLogWriter {
	var w = &SocketLogWriter{}

	w.LogCloserInit()

	sock, err := net.Dial(proto, hostport)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewSocketLogWriter(connect %q): %s\n", hostport, err)
		return nil
	}

	w.rec = make(chan LogRecord, LogBufferLength)

	go func() {
		defer func() {
			//if sock != nil && proto == "tcp" {
			if sock != nil {
				sock.Close()
			}
		}()

		for rec := range w.rec {
			// Marshall into JSON
			//js, err := json.Marshal(rec)
			//if err != nil {
			//	//fmt.Fprint(os.Stderr, "SocketLogWriter(%s): %s", hostport, err)
			//	errStr := fmt.Sprintf("SocketLogWriter(%s): %s", hostport, err)
			//	fmt.Fprint(os.Stderr, errStr)
			//	return
			//}

			if w.IsClosed(rec) {
				return
			}

			if !w.caller {
				rec.Source = ""
			}

			//recJson, _ := json.Marshal(rec)
			_, err = sock.Write(rec.JSON())
			if err != nil {
				errStr := fmt.Sprintf("SocketLogWriter(%q): %s", hostport, err)
				fmt.Fprint(os.Stderr, errStr)
				if proto == "udp" {
					// retry if send failed to send udp datagram packet
					time.Sleep(SockFailWaitTimeout)
				} else {
					return
				}
			}
		}
	}()

	return w
}

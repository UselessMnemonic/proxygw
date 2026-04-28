package ipc

import (
	"errors"
	"sync"

	"github.com/UselessMnemonic/proxygw/plugin/ctl/ipc/method"
)

// BaseServer is a one-way IPC server that receives requests and sends
// responses.
type BaseServer struct {
	conn   *Conn
	notifs chan Packet
	reqs   chan Packet
	inbox  map[uint32]struct{}
	lock   sync.Mutex
}

// NewBaseServer constructs a BaseServer around conn and starts its read loop.
func NewBaseServer(conn *Conn) *BaseServer {
	bs := &BaseServer{
		conn:   conn,
		notifs: make(chan Packet, 16),
		reqs:   make(chan Packet, 16),
		inbox:  make(map[uint32]struct{}),
		lock:   sync.Mutex{},
	}
	bs.start()
	return bs
}

// Close closes the underlying connection. Pending operations will asynchronously close.
func (bs *BaseServer) Close() error {
	return bs.conn.Close()
}

// Notifications returns incoming notification packets.
func (bs *BaseServer) Notifications() <-chan Packet {
	return bs.notifs
}

// Requests returns incoming request packets that require a response.
func (bs *BaseServer) Requests() <-chan Packet {
	return bs.reqs
}

// Notify sends notif as a notification by forcing its ID to zero.
func (bs *BaseServer) Notify(notif *Packet) error {
	notif.Id = 0
	bs.lock.Lock()
	defer bs.lock.Unlock()
	return bs.conn.Write(notif)
}

// Respond sends res for a pending incoming request and clears its tracking state.
func (bs *BaseServer) Respond(res Packet) error {
	if res.Id == 0 {
		return errors.New("response id should not be zero")
	}
	if !method.IsResponse(res.Method) {
		return errors.New("cannot respond with a request")
	}
	bs.lock.Lock()
	defer bs.lock.Unlock()
	if _, exists := bs.inbox[res.Id]; !exists {
		return errors.New("no incoming request is pending this response")
	}

	err := bs.conn.Write(&res)
	if err != nil {
		return err
	}
	delete(bs.inbox, res.Id)
	return nil
}

func (bs *BaseServer) end() {
	bs.lock.Lock()
	defer bs.lock.Unlock()
	close(bs.notifs)
	close(bs.reqs)
}

func (bs *BaseServer) dispatchReq(req Packet) error {
	bs.lock.Lock()
	defer bs.lock.Unlock()
	if _, exists := bs.inbox[req.Id]; exists {
		errPacket := MakePacket(req.Id, method.ErrorResponse{Message: "existing request already in flight"})
		return bs.conn.Write(&errPacket)
	}
	bs.inbox[req.Id] = struct{}{}
	bs.reqs <- req
	return nil
}

func (bs *BaseServer) start() {
	go func() {
		defer bs.end()
		for {
			p, err := bs.conn.Read()
			if err != nil {
				return
			}
			if p.Id != 0 && method.IsResponse(p.Method) {
				continue
			}
			if err := ParseRawBody(bs.conn.codec, &p); err != nil {
				return
			}
			if p.Id == 0 {
				bs.notifs <- p
				continue
			}
			if err := bs.dispatchReq(p); err != nil {
				return
			}
		}
	}()
}

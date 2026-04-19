package ipc

import (
	"errors"
	"proxygw/pkg/ipc/codec"
	"proxygw/pkg/ipc/method"
	"sync"
)

type BasePeer struct {
	conn   *Conn
	notifs chan Packet
	reqs   chan Packet
	outbox map[uint32]chan<- Packet
	inbox  map[uint32]struct{}
	lock   sync.Mutex
	nextId uint32
}

// NewBasePeer constructs a BasePeer around conn and starts its read loop.
func NewBasePeer(conn *Conn) *BasePeer {
	bp := &BasePeer{
		conn:   conn,
		notifs: make(chan Packet, 16),
		reqs:   make(chan Packet, 16),
		outbox: make(map[uint32]chan<- Packet),
		inbox:  make(map[uint32]struct{}),
		lock:   sync.Mutex{},
		nextId: 0,
	}
	bp.start()
	return bp
}

// Close closes the underlying connection. Pending operations will asynchronously close.
func (bp *BasePeer) Close() error {
	return bp.conn.Close()
}

// Notifications returns incoming notification packets.
func (bp *BasePeer) Notifications() <-chan Packet {
	return bp.notifs
}

// Requests returns incoming request packets that require a response.
func (bp *BasePeer) Requests() <-chan Packet {
	return bp.reqs
}

// Notify sends notif as a notification by forcing its ID to zero.
func (bp *BasePeer) Notify(notif *Packet) error {
	notif.Id = 0
	bp.lock.Lock()
	defer bp.lock.Unlock()
	return bp.conn.Write(notif)
}

// Request sends req with a new request ID and returns a channel for its response.
func (bp *BasePeer) Request(req *Packet) (<-chan Packet, error) {
	bp.lock.Lock()
	defer bp.lock.Unlock()
	req.Id = bp.assignId()
	if _, exists := bp.outbox[req.Id]; exists {
		return nil, errors.New("request id already in use")
	}

	ch := make(chan Packet, 1)
	bp.outbox[req.Id] = ch
	err := bp.conn.Write(req)
	if err != nil {
		delete(bp.outbox, req.Id)
		close(ch)
		return nil, err
	}

	return ch, nil
}

// Respond sends res for a pending incoming request and clears its tracking state.
func (bp *BasePeer) Respond(res Packet) error {
	if res.Id == 0 {
		return errors.New("response id should not be zero")
	}
	bp.lock.Lock()
	defer bp.lock.Unlock()
	if _, exists := bp.inbox[res.Id]; !exists {
		return errors.New("no incoming request is pending this response")
	}

	err := bp.conn.Write(&res)
	if err != nil {
		return err
	}
	delete(bp.inbox, res.Id)
	return nil
}

func (bp *BasePeer) assignId() uint32 {
	bp.nextId++
	if bp.nextId == 0 {
		bp.nextId = 1
	}
	return bp.nextId
}

func (bp *BasePeer) end() {
	bp.lock.Lock()
	defer bp.lock.Unlock()
	for _, ch := range bp.outbox {
		close(ch)
	}
	close(bp.notifs)
	close(bp.reqs)
}

func (bp *BasePeer) dispatchReq(req Packet) error {
	bp.lock.Lock()
	defer bp.lock.Unlock()
	_, exists := bp.inbox[req.Id]
	if exists {
		errPacket := MakePacket(req.Id, method.ErrorResponse{"existing request already in flight"})
		return bp.conn.Write(&errPacket)
	}
	if err := ParseRawPacket(bp.conn.codec, &req); err != nil {
		return err
	}
	bp.inbox[req.Id] = struct{}{}
	bp.reqs <- req
	return nil
}

func (bp *BasePeer) matchResp(res Packet) {
	bp.lock.Lock()
	defer bp.lock.Unlock()
	ch, exists := bp.outbox[res.Id]
	if !exists {
		return
	}
	delete(bp.outbox, res.Id)
	ch <- res
}

func (bp *BasePeer) start() {
	go func() {
		defer bp.end()
		for {
			p, err := bp.conn.Read()
			if err != nil {
				return
			}
			// emit notification
			if p.Id == 0 {
				bp.notifs <- p
			}
			// dispatch request
			if p.Method%2 == 0 {
				err = bp.dispatchReq(p)
				if err != nil {
					return
				}
			}
			// match outbox
			bp.matchResp(p)
		}
	}()
}

// MakePacket builds a Packet from an ID and method body.
func MakePacket[M method.Method](id uint32, body M) Packet {
	return Packet{
		Id:     id,
		Method: body.Method(),
		Body:   body,
	}
}

func ParseRawPacket(codec codec.Codec, packet *Packet) error {
	raw := packet.Body.([]byte)
	switch packet.Method {
	case method.MethodEmpty:
		packet.Body = &method.Empty{}
		return nil
	case method.MethodErrorResponse:
		packet.Body = &method.ErrorResponse{}
	case method.MethodStatusRequest:
		packet.Body = &method.StatusRequest{}
	case method.MethodStatusResponse:
		packet.Body = &method.StatusResponse{}
	default:
		return nil
	}
	return codec.Unmarshal(raw, packet.Body)
}

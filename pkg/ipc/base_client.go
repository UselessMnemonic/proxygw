package ipc

import (
	"errors"
	"proxygw/pkg/ipc/method"
	"sync"
)

type BaseClient struct {
	conn   *Conn
	notifs chan Packet
	outbox map[uint32]chan<- Packet
	lock   sync.Mutex
	nextId uint32
}

// NewBaseClient constructs a BaseClient around conn and starts its read loop.
func NewBaseClient(conn *Conn) *BaseClient {
	bc := &BaseClient{
		conn:   conn,
		notifs: make(chan Packet, 16),
		outbox: make(map[uint32]chan<- Packet),
		lock:   sync.Mutex{},
		nextId: 0,
	}
	bc.start()
	return bc
}

// Close closes the underlying connection. Pending operations will asynchronously close.
func (bc *BaseClient) Close() error {
	return bc.conn.Close()
}

// Notifications returns incoming notification packets.
func (bc *BaseClient) Notifications() <-chan Packet {
	return bc.notifs
}

// Notify sends notif as a notification by forcing its ID to zero.
func (bc *BaseClient) Notify(notif *Packet) error {
	notif.Id = 0
	bc.lock.Lock()
	defer bc.lock.Unlock()
	return bc.conn.Write(notif)
}

// Request sends req with a new request ID and returns a channel for its response.
func (bc *BaseClient) Request(req *Packet) (<-chan Packet, error) {
	if method.IsResponse(req.Method) {
		return nil, errors.New("cannot send a response")
	}

	bc.lock.Lock()
	defer bc.lock.Unlock()
	req.Id = bc.assignId()
	if _, exists := bc.outbox[req.Id]; exists {
		return nil, errors.New("request id already in use")
	}

	ch := make(chan Packet, 1)
	bc.outbox[req.Id] = ch
	err := bc.conn.Write(req)
	if err != nil {
		delete(bc.outbox, req.Id)
		close(ch)
		return nil, err
	}

	return ch, nil
}

func (bc *BaseClient) assignId() uint32 {
	bc.nextId++
	if bc.nextId == 0 {
		bc.nextId = 1
	}
	return bc.nextId
}

func (bc *BaseClient) end() {
	bc.lock.Lock()
	defer bc.lock.Unlock()
	for _, ch := range bc.outbox {
		close(ch)
	}
	close(bc.notifs)
}

func (bc *BaseClient) matchResp(res Packet) {
	bc.lock.Lock()
	defer bc.lock.Unlock()
	ch, exists := bc.outbox[res.Id]
	if !exists {
		return
	}
	delete(bc.outbox, res.Id)
	ch <- res
}

func (bc *BaseClient) start() {
	go func() {
		defer bc.end()
		for {
			p, err := bc.conn.Read()
			if err != nil {
				return
			}
			if p.Id != 0 && !method.IsResponse(p.Method) {
				continue
			}
			if err := ParseRawBody(bc.conn.codec, &p); err != nil {
				return
			}
			if p.Id == 0 {
				bc.notifs <- p
				continue
			}
			bc.matchResp(p)
		}
	}()
}

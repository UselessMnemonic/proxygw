package ipc

type Packet struct {
	Id     uint32 `json:"id"`
	Method uint16 `json:"method"`
	Body   any    `json:"body"`
}

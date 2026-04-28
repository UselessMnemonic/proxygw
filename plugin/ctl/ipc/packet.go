package ipc

// Packet is the transport envelope for control IPC messages.
type Packet struct {
	Id     uint32 `json:"id"`
	Method uint16 `json:"method"`
	Body   any    `json:"body"`
}

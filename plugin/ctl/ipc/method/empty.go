package method

// Empty is used when a message carries no fields.
type Empty struct{}

func (Empty) Method() uint16 {
	return MethodEmpty
}

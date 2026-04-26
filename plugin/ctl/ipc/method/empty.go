package method

type Empty struct{}

func (Empty) Method() uint16 {
	return MethodEmpty
}

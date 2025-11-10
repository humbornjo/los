package regexp

type opcode int

const (
	OP_CHAR opcode = iota
	OP_MATCH
	OP_JMP
	OP_SPLIT
)

package regexp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMachine_Match(t *testing.T) {
	re, err := Compile("abc")
	require.NoError(t, err)

	machine := re.Get()
	fmt.Println(machine.Match("a"))
	fmt.Println(machine.Match("b"))
	fmt.Println(machine.Match("c"))
	re.Put(machine)
}

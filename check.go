package main

import (
	"fmt"
	"github.com/byteme/compiler/object"
)

func main() {
	fmt.Println("Total builtins:", len(object.Builtins))
}

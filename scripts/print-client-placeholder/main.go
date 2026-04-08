package main

import (
	"fmt"
	"os"

	"github.com/pizixi/gpipe/internal/clientbin"
)

func main() {
	if _, err := fmt.Fprint(os.Stdout, clientbin.PlaceholderValue()); err != nil {
		panic(err)
	}
}

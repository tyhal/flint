package main

import (
	"fmt"
	"os"

	"github.com/tyhal/flint/flint"
)

func main() {
	app := flint.NewApp()

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

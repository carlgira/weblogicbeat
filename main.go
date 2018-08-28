package main

import (
	"os"

	"github.com/carlgira/weblogicbeat/cmd"

	_ "github.com/carlgira/weblogicbeat/include"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

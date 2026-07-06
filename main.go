package main

import (
	"os"

	"github.com/talby/talby-bootstrap/cmd/tbboot"
)

func main() {
	os.Exit(tbboot.Execute())
}

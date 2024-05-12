/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"os"

	"github.com/theshashankpal/trident_debug/cmd"
)

func main() {

	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rdark/cdagee/target"
)

func runValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	cf := addCommonFlags(fs)
	_ = fs.Parse(args) // ExitOnError: exits before returning error
	resolveOutput(cf, fs)
	checkNoArgs(fs)

	result, err := target.Discover(cf.root)
	if err != nil {
		fatalf("validate: %v", err)
	}

	validationErr := target.Validate(result.Targets)

	out := struct {
		Valid  bool     `json:"valid"`
		Errors []string `json:"errors,omitempty"`
	}{
		Valid:  validationErr == nil,
		Errors: splitErrors(validationErr),
	}
	if cf.writeOutput(out) {
		if validationErr != nil {
			os.Exit(1)
		}
		return
	}

	if validationErr != nil {
		fatalf("validate: %v", validationErr)
	}
	fmt.Println("ok")
}

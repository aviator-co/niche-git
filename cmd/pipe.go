// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/spf13/cobra"
)

var pipeArg struct {
	command    string
	inputFile  string
	outputFile string
}

var pipeCmd = &cobra.Command{
	Use: "pipe",
	RunE: func(cmd *cobra.Command, args []string) error {
		var in io.Reader
		if pipeArg.inputFile == "-" {
			in = os.Stdin
		} else {
			file, err := os.Open(pipeArg.inputFile)
			if err != nil {
				return err
			}
			defer file.Close()
			in = file
		}
		dec := json.NewDecoder(in)

		switch pipeArg.command {
		case "get-files":
			input := GetFilesArgs{}
			if err := dec.Decode(&input); err != nil {
				return err
			}
			output := GetFiles(input)
			return writeJSON(pipeArg.outputFile, output)
		case "get-modified-files-regexp-matches":
			input := GetModifiedFilesRegexpMatchesArgs{}
			if err := dec.Decode(&input); err != nil {
				return err
			}
			output := GetModifiedFilesRegexpMatches(input)
			return writeJSON(pipeArg.outputFile, output)
		case "squash-push":
			input := nichegit.SquashPushArgs{}
			if err := dec.Decode(&input); err != nil {
				return err
			}
			output := SquashPush(input)
			return writeJSON(pipeArg.outputFile, output)
		case "update-refs":
			input := nichegit.UpdateRefsArgs{}
			if err := dec.Decode(&input); err != nil {
				return err
			}
			output := UpdateRefs(input)
			return writeJSON(pipeArg.outputFile, output)
		case "backport":
			input := nichegit.BackportArgs{}
			if err := dec.Decode(&input); err != nil {
				return err
			}
			output := Backport(input)
			return writeJSON(pipeArg.outputFile, output)
		}

		return fmt.Errorf("unknown command: %s", pipeArg.command)
	},
}

func init() {
	rootCmd.AddCommand(pipeCmd)

	pipeCmd.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	pipeCmd.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	pipeCmd.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	pipeCmd.Flags().StringVar(&pipeArg.command, "command", "", "Command to execute")
	pipeCmd.Flags().StringVar(&pipeArg.inputFile, "input-file", "-", "Optional input file path. '-', which is the default, means stdin")
	pipeCmd.Flags().StringVar(&pipeArg.outputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
	_ = pipeCmd.MarkFlagRequired("command")
}

// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"
	"time"

	nichegit "github.com/aviator-co/niche-git"
	"github.com/aviator-co/niche-git/debug"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/spf13/cobra"
)

var (
	squashCherryPickArgs struct {
		repoURL         string
		cherryPickFrom  string
		cherryPickTo    string
		cherryPickBase  string
		commitMessage   string
		author          string
		authorEmail     string
		authorTime      string
		committer       string
		committerEmail  string
		committerTime   string
		ref             string
		conflictRef     string
		currentRefHash  string
		abortOnConflict bool

		outputFile string
	}
)

var squashCherryPick = &cobra.Command{
	Use: "squash-cherry-pick",
	RunE: func(cmd *cobra.Command, args []string) error {
		var currentRefhash *plumbing.Hash
		if squashCherryPickArgs.currentRefHash != "" {
			hash := plumbing.NewHash(squashCherryPickArgs.currentRefHash)
			currentRefhash = &hash
		}
		author, err := newSignature(squashCherryPickArgs.author, squashCherryPickArgs.authorEmail, squashCherryPickArgs.authorTime)
		if err != nil {
			return err
		}
		committer, err := newSignature(squashCherryPickArgs.committer, squashCherryPickArgs.committerEmail, squashCherryPickArgs.committerTime)
		if err != nil {
			return err
		}

		client := &http.Client{Transport: &authnRoundtripper{}}
		var conflictRef *plumbing.ReferenceName
		if squashCherryPickArgs.conflictRef != "" {
			r := plumbing.ReferenceName(squashCherryPickArgs.conflictRef)
			conflictRef = &r
		}
		result, fetchDebugInfo, blobFetchDebugInfo, pushDebugInfo, pushErr := nichegit.PushSquashCherryPick(
			squashCherryPickArgs.repoURL,
			client,
			plumbing.NewHash(squashCherryPickArgs.cherryPickFrom),
			plumbing.NewHash(squashCherryPickArgs.cherryPickTo),
			plumbing.NewHash(squashCherryPickArgs.cherryPickBase),
			squashCherryPickArgs.commitMessage,
			author,
			committer,
			plumbing.ReferenceName(squashCherryPickArgs.ref),
			conflictRef,
			currentRefhash,
			squashCherryPickArgs.abortOnConflict,
		)
		output := squashCherryPickOutput{
			FetchDebugInfo:     fetchDebugInfo,
			BlobFetchDebugInfo: blobFetchDebugInfo,
			PushDebugInfo:      pushDebugInfo,
		}
		if result != nil {
			output.CommitHash = result.CommitHash.String()
			output.CherryPickedFiles = result.CherryPickedFiles
			output.ConflictOpenFiles = result.ConflictOpenFiles
			output.ConflictResolvedFiles = result.ConflictResolvedFiles
			output.BinaryConflictFiles = result.BinaryConflictFiles
			output.NonFileConflictFiles = result.NonFileConflictFiles
		}
		if output.CherryPickedFiles == nil {
			output.CherryPickedFiles = []string{}
		}
		if output.ConflictOpenFiles == nil {
			output.ConflictOpenFiles = []string{}
		}
		if output.ConflictResolvedFiles == nil {
			output.ConflictResolvedFiles = []string{}
		}
		if output.BinaryConflictFiles == nil {
			output.BinaryConflictFiles = []string{}
		}
		if output.NonFileConflictFiles == nil {
			output.NonFileConflictFiles = []string{}
		}
		if pushErr != nil {
			output.Error = pushErr.Error()
		}
		if err := writeJSON(squashCherryPickArgs.outputFile, output); err != nil {
			return err
		}
		return pushErr
	},
}

func newSignature(name, email, timestamp string) (object.Signature, error) {
	var t time.Time
	if timestamp == "" {
		t = time.Now()
	} else {
		var err error
		t, err = time.Parse(time.RFC3339, timestamp)
		if err != nil {
			return object.Signature{}, err
		}
	}
	return object.Signature{
		Name:  name,
		Email: email,
		When:  t,
	}, nil
}

type squashCherryPickOutput struct {
	CommitHash            string                `json:"commitHash"`
	CherryPickedFiles     []string              `json:"cherryPickedFiles"`
	ConflictOpenFiles     []string              `json:"conflictOpenFiles"`
	ConflictResolvedFiles []string              `json:"conflictResolvedFiles"`
	BinaryConflictFiles   []string              `json:"binaryConflictFiles"`
	NonFileConflictFiles  []string              `json:"nonFileConflictFiles"`
	FetchDebugInfo        debug.FetchDebugInfo  `json:"fetchDebugInfo"`
	BlobFetchDebugInfo    *debug.FetchDebugInfo `json:"blobFetchDebugInfo"`
	PushDebugInfo         *debug.PushDebugInfo  `json:"pushDebugInfo"`
	Error                 string                `json:"error,omitempty"`
}

func init() {
	rootCmd.AddCommand(squashCherryPick)
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.repoURL, "repo-url", "", "Git repository URL")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.cherryPickFrom, "cherry-pick-from", "", "Commit hash where cherry-pick from")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.cherryPickTo, "cherry-pick-to", "", "Commit hash where cherry-pick to")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.cherryPickBase, "cherry-pick-base", "", "The merge base of the cherry-pick from. The changes from this commit to cherry-pick-from will be applied to cherry-pick-to.")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.commitMessage, "commit-message", "", "Commit message of the squashed commit")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.author, "author", "", "Author name")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.authorEmail, "author-email", "", "Author email address")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.authorTime, "author-time", "", "Author time in RFC3339 format (e.g. 2024-01-01T00:00:00Z)")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.committer, "committer", "", "Committer name")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.committerEmail, "committer-email", "", "Committer email address")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.committerTime, "committer-time", "", "Commit time in RFC3339 format (e.g. 2024-01-01T00:00:00Z)")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.ref, "ref", "", "A ref name (e.g. refs/heads/foobar) to push")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.conflictRef, "conflict-ref", "", "A ref name (e.g. refs/heads/foobar) to push when there's a conflict")
	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.currentRefHash, "current-ref-hash", "", "The expected current commit hash of the ref. If this is specified, the push will use the current commit hash. This is used for compare-and-swap.")
	squashCherryPick.Flags().BoolVar(&squashCherryPickArgs.abortOnConflict, "abort-on-conflict", false, "Abort the operation if there is a merge conflict")
	_ = squashCherryPick.MarkFlagRequired("repo-url")
	_ = squashCherryPick.MarkFlagRequired("cherry-pick-from")
	_ = squashCherryPick.MarkFlagRequired("cherry-pick-to")
	_ = squashCherryPick.MarkFlagRequired("cherry-pick-base")
	_ = squashCherryPick.MarkFlagRequired("commit-message")
	_ = squashCherryPick.MarkFlagRequired("author")
	_ = squashCherryPick.MarkFlagRequired("author-email")
	_ = squashCherryPick.MarkFlagRequired("committer")
	_ = squashCherryPick.MarkFlagRequired("committer-email")
	_ = squashCherryPick.MarkFlagRequired("ref")

	squashCherryPick.Flags().StringVar(&authzHeader, "authz-header", "", "Optional authorization header")
	squashCherryPick.Flags().StringVar(&basicAuthzUser, "basic-authz-user", "", "Optional HTTP Basic Auth user")
	squashCherryPick.Flags().StringVar(&basicAuthzPassword, "basic-authz-password", "", "Optional HTTP Basic Auth password")

	squashCherryPick.Flags().StringVar(&squashCherryPickArgs.outputFile, "output-file", "-", "Optional output file path. '-', which is the default, means stdout")
}

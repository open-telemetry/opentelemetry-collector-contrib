// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/opentelemetry/opentelemetry-log-collection/database"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/helper"
	"github.com/spf13/cobra"
	"go.etcd.io/bbolt"
)

var stdout io.Writer = os.Stdout

// NewOffsetsCmd returns the root command for managing offsets
func NewOffsetsCmd(rootFlags *RootFlags) *cobra.Command {
	offsets := &cobra.Command{
		Use:   "offsets",
		Short: "Manage input operator offsets",
		Args:  cobra.NoArgs,
		Run: func(command *cobra.Command, args []string) {
			stdout.Write([]byte("No offsets subcommand specified. See `stanza offsets help` for details\n"))
		},
	}

	offsets.AddCommand(NewOffsetsClearCmd(rootFlags))
	offsets.AddCommand(NewOffsetsListCmd(rootFlags))

	return offsets
}

// NewOffsetsClearCmd returns the command for clearing offsets
func NewOffsetsClearCmd(rootFlags *RootFlags) *cobra.Command {
	var all bool

	offsetsClear := &cobra.Command{
		Use:   "clear [flags] [operator_ids]",
		Short: "Clear persisted offsets from the database",
		Args:  cobra.ArbitraryArgs,
		Run: func(command *cobra.Command, args []string) {
			db, err := database.OpenDatabase(rootFlags.DatabaseFile)
			exitOnErr("Failed to open database", err)
			defer db.Close()
			defer func() { _ = db.Sync() }()

			if all {
				if len(args) != 0 {
					stdout.Write([]byte("Providing a list of operator IDs does nothing with the --all flag\n"))
				}

				err := db.Update(func(tx *bbolt.Tx) error {
					offsetsBucket := tx.Bucket(helper.OffsetsBucket)
					if offsetsBucket != nil {
						return tx.DeleteBucket(helper.OffsetsBucket)
					}
					return nil
				})
				exitOnErr("Failed to delete offsets", err)
			} else {
				if len(args) == 0 {
					stdout.Write([]byte("Must either specify a list of operators or the --all flag\n"))
					os.Exit(1)
				}

				for _, operatorID := range args {
					err = db.Update(func(tx *bbolt.Tx) error {
						offsetBucket := tx.Bucket(helper.OffsetsBucket)
						if offsetBucket == nil {
							return nil
						}

						return offsetBucket.DeleteBucket([]byte(operatorID))
					})
					exitOnErr("Failed to delete offsets", err)
				}
			}
		},
	}

	offsetsClear.Flags().BoolVar(&all, "all", false, "clear offsets for all inputs")

	return offsetsClear
}

// NewOffsetsListCmd returns the command for listing offsets
func NewOffsetsListCmd(rootFlags *RootFlags) *cobra.Command {
	offsetsList := &cobra.Command{
		Use:   "list",
		Short: "List operators with persisted offsets",
		Args:  cobra.NoArgs,
		Run: func(command *cobra.Command, args []string) {
			db, err := database.OpenDatabase(rootFlags.DatabaseFile)
			exitOnErr("Failed to open database", err)
			defer db.Close()

			err = db.View(func(tx *bbolt.Tx) error {
				offsetBucket := tx.Bucket(helper.OffsetsBucket)
				if offsetBucket == nil {
					return nil
				}

				return offsetBucket.ForEach(func(key, value []byte) error {
					stdout.Write(append(key, '\n'))
					return nil
				})
			})
			if err != nil {
				exitOnErr("Failed to read database", err)
			}
		},
	}

	return offsetsList
}

func exitOnErr(msg string, err error) {
	if err != nil {
		os.Stderr.WriteString(fmt.Sprintf("%s: %s\n", msg, err))
		os.Exit(1)
	}
}

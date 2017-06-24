// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"

	cli "github.com/spf13/cobra"
)

// rateLimit represents the ratelimit command
var rateLimitCmd = &cli.Command{
	Use:   "ratelimit",
	Short: "check your API rate limit",
	Run: func(cmd *cli.Command, args []string) {
		if err := runRateLimit(cmd, args); err != nil {
			cmd.Println(err)
		}
	},
}

func init() {
	RootCmd.AddCommand(rateLimitCmd)

	rateLimitCmd.Flags().String("token", "", "GitHub Personal access token")
}

func runRateLimit(cmd *cli.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newClient(ctx)
	rateLimit, _, err := client.RateLimits(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Your rate limit: %d, Remaining: %d\n", rateLimit.Core.Limit, rateLimit.Core.Remaining)
	fmt.Printf("Reset time: %v", rateLimit.Core.Reset)

	return nil
}

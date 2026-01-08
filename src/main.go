/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package main

import (
	"context"
	"log"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/humaidq/groundwave/cmd"
)

func main() {
	app := &cli.Command{
		Name:  "groundwave",
		Usage: "Groundwave - Personal Database",
		Commands: []*cli.Command{
			cmd.CmdStart,
			cmd.CmdMigrate,
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

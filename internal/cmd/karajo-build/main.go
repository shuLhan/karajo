// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

// Package karajo-build provide internal commands to build karajo for
// development.
package main

import (
	"flag"
	"log"
	"strings"

	"github.com/shuLhan/share/lib/memfs"

	"git.sr.ht/~shulhan/karajo/internal"
)

// List of build commands.
const (
	cmdEmbed = `embed`
)

func main() {
	flag.Parse()

	var (
		cmd = flag.Arg(0)

		err error
	)

	cmd = strings.ToLower(cmd)

	switch cmd {
	case cmdEmbed:
		err = internal.ConvertAdocToHtml()
		if err != nil {
			log.Fatalf(err.Error())
		}

		var mfs *memfs.MemFS

		mfs, err = internal.GenerateMemfs()
		if err != nil {
			log.Fatalf(err.Error())
		}

		err = mfs.GoEmbed()
		if err != nil {
			log.Fatalf(err.Error())
		}
		return
	default:
		log.Printf(`unknown command: %s`, cmd)
	}
}

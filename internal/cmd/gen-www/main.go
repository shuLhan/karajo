// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"log"

	"github.com/shuLhan/share/lib/memfs"
)

func main() {
	opts := memfs.Options{
		Root: "_www",
		Embed: memfs.EmbedOptions{
			CommentHeader: `// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later
`,
			PackageName: "karajo",
			VarName:     "memfsWww",
			GoFileName:  "memfs_www.go",
		},
	}

	mfs, err := memfs.New(&opts)
	if err != nil {
		log.Fatal(err)
	}
	err = mfs.GoEmbed()
	if err != nil {
		log.Fatal(err)
	}
}

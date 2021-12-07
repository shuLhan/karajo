// Copyright 2021, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"log"

	"github.com/shuLhan/share/lib/memfs"
)

func main() {
	opts := memfs.Options{
		Root: "_www",
		Embed: memfs.EmbedOptions{
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

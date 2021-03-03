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
	}

	mfs, err := memfs.New(&opts)
	if err != nil {
		log.Fatal(err)
	}
	err = mfs.GoGenerate("karajo", "memfsWww", "memfs_www.go", "")
	if err != nil {
		log.Fatal(err)
	}
}

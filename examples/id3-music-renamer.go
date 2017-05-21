// Copyright 2017 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a Modified
// BSD License that can be found in the LICENSE file.

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tmthrgd/id3v2"
)

var dryrun = flag.Bool("dry-run", false, "does not perform the file renaming")

func scan(work workUnit) error {
	f, err := os.Open(work.path)
	if err != nil {
		return err
	}
	defer f.Close()

	frames, err := id3v2.Scan(f)
	if err != nil {
		return err
	}

	tit2 := frames.Lookup(id3v2.FrameTIT2)
	tpe1 := frames.Lookup(id3v2.FrameTPE1)

	if tit2 == nil || tpe1 == nil {
		if filepath.Ext(work.path) != ".mp3" {
			return nil
		}

		return errors.New("missing TIT2 or TPE1 frame")
	}

	title, err := tit2.Text()
	if err != nil {
		return err
	}

	artist, err := tpe1.Text()
	if err != nil {
		return err
	}

	newName := title + " - " + artist + filepath.Ext(work.path)
	newName = strings.Replace(newName, string(filepath.Separator), "-", -1)

	newPath := filepath.Join(filepath.Dir(work.path), newName)

	if work.path == newPath {
		return nil
	}

	newURL := (&url.URL{
		Scheme: "file",
		Path:   newPath,
	}).String()
	*work.out = newURL

	name, padding := filepath.Base(work.path), " "
	if len(name) < 100 {
		padding = strings.Repeat(" ", 100-len(name))
	}

	fmt.Printf("%s%s%s\n", name, padding, filepath.Base(newPath))

	if *dryrun {
		return nil
	}

	return os.Rename(work.path, newPath)
}

func worker(ch chan workUnit, wg *sync.WaitGroup) {
	for work := range ch {
		if err := scan(work); err != nil {
			fmt.Fprintf(os.Stderr, "<%s>: %v\n", work.path, err)
		}

		wg.Done()
	}
}

type workUnit struct {
	path string
	out  *string
}

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Println("Usage: id3-music-renamer.go [-dry-run] <m3u8 playlist> [<m3u8 playlist out>]")
		os.Exit(1)
	}

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		panic(err)
	}
	defer r.Close()

	var wg sync.WaitGroup

	work := make(chan workUnit, 32)
	defer close(work)

	for i := 0; i < cap(work); i++ {
		go worker(work, &wg)
	}

	s := bufio.NewScanner(r)

	var out []*string

	for s.Scan() {
		line := s.Text()

		path, err := url.Parse(line)
		if err != nil {
			panic(err)
		}

		out = append(out, &line)

		if path.Scheme != "file" {
			continue
		}

		wg.Add(1)
		work <- workUnit{path.Path, out[len(out)-1]}
	}

	if s.Err() != nil {
		panic(s.Err())
	}

	wg.Wait()

	var outPath string
	if flag.NArg() == 2 {
		outPath = flag.Arg(1)
	} else {
		outPath = flag.Arg(0) + "-new.m3u8"
	}

	w, err := os.Create(outPath)
	if err != nil {
		panic(err)
	}
	defer w.Close()

	bw := bufio.NewWriter(w)

	for _, line := range out {
		if _, err := io.WriteString(bw, *line); err != nil {
			panic(err)
		}

		if _, err := io.WriteString(bw, "\n"); err != nil {
			panic(err)
		}
	}

	if err := bw.Flush(); err != nil {
		panic(err)
	}
}

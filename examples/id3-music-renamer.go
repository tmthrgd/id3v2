// Copyright 2017 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a Modified
// BSD License that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
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

	tpe1 := frames.Lookup(id3v2.FrameTPE1)
	if tpe1 == nil {
		if filepath.Ext(work.path) == ".mp3" {
			fmt.Printf("<%s>: missing TPE1 frame\n",
				work.path)
		}

		return nil
	}

	artist, err := tpe1.Text()
	if err != nil {
		return err
	}

	ext := filepath.Ext(work.path)
	if strings.HasSuffix(strings.TrimSuffix(work.path, ext), artist) {
		return nil
	}

	newPath := strings.TrimSuffix(work.path, ext) + " - " + artist + ext

	newURL := (&url.URL{
		Scheme: "file",
		Path:   newPath,
	}).String()
	*work.out = newURL

	fmt.Printf("%s: %s\n", filepath.Base(work.path), artist)

	if *dryrun {
		return nil
	}

	return os.Rename(work.path, newPath)
}

func worker(ch chan workUnit, wg *sync.WaitGroup) {
	for work := range ch {
		if err := scan(work); err != nil {
			log.Println(work.path, err)
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

	if flag.NArg() != 1 {
		fmt.Println("Usage: id3-music-renamer.go [-dry-run] <m3u8 playlist>")
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

	w, err := os.Create(flag.Arg(0) + ".new.m3u8")
	if err != nil {
		panic(err)
	}
	defer w.Close()

	bw := bufio.NewWriter(w)
	defer bw.Flush()

	for _, line := range out {
		if _, err := io.WriteString(bw, *line); err != nil {
			panic(err)
		}

		if _, err := io.WriteString(bw, "\n"); err != nil {
			panic(err)
		}
	}
}

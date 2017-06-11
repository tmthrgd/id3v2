// Copyright 2017 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a Modified
// BSD License that can be found in the LICENSE file.

//+build ignore

package main

import (
	"bufio"
	"flag"
	"os"
	"strings"
	"text/template"
)

// Taken from http://id3.org/id3v2.4.0-frames.
//  id3v2.4.0-frames.txt,v 1.1 2003/07/27 18:28:34 id3 Exp
const v24Spec = `
  4.19  AENC Audio encryption
  4.14  APIC Attached picture
  4.30  ASPI Audio seek point index

  4.10  COMM Comments
  4.24  COMR Commercial frame

  4.25  ENCR Encryption method registration
  4.12  EQU2 Equalisation (2)
  4.5   ETCO Event timing codes

  4.15  GEOB General encapsulated object
  4.26  GRID Group identification registration

  4.20  LINK Linked information

  4.4   MCDI Music CD identifier
  4.6   MLLT MPEG location lookup table

  4.23  OWNE Ownership frame

  4.27  PRIV Private frame
  4.16  PCNT Play counter
  4.17  POPM Popularimeter
  4.21  POSS Position synchronisation frame

  4.18  RBUF Recommended buffer size
  4.11  RVA2 Relative volume adjustment (2)
  4.13  RVRB Reverb

  4.29  SEEK Seek frame
  4.28  SIGN Signature frame
  4.9   SYLT Synchronised lyric/text
  4.7   SYTC Synchronised tempo codes

  4.2.1 TALB Album/Movie/Show title
  4.2.3 TBPM BPM (beats per minute)
  4.2.2 TCOM Composer
  4.2.3 TCON Content type
  4.2.4 TCOP Copyright message
  4.2.5 TDEN Encoding time
  4.2.5 TDLY Playlist delay
  4.2.5 TDOR Original release time
  4.2.5 TDRC Recording time
  4.2.5 TDRL Release time
  4.2.5 TDTG Tagging time
  4.2.2 TENC Encoded by
  4.2.2 TEXT Lyricist/Text writer
  4.2.3 TFLT File type
  4.2.2 TIPL Involved people list
  4.2.1 TIT1 Content group description
  4.2.1 TIT2 Title/songname/content description
  4.2.1 TIT3 Subtitle/Description refinement
  4.2.3 TKEY Initial key
  4.2.3 TLAN Language(s)
  4.2.3 TLEN Length
  4.2.2 TMCL Musician credits list
  4.2.3 TMED Media type
  4.2.3 TMOO Mood
  4.2.1 TOAL Original album/movie/show title
  4.2.5 TOFN Original filename
  4.2.2 TOLY Original lyricist(s)/text writer(s)
  4.2.2 TOPE Original artist(s)/performer(s)
  4.2.4 TOWN File owner/licensee
  4.2.2 TPE1 Lead performer(s)/Soloist(s)
  4.2.2 TPE2 Band/orchestra/accompaniment
  4.2.2 TPE3 Conductor/performer refinement
  4.2.2 TPE4 Interpreted, remixed, or otherwise modified by
  4.2.1 TPOS Part of a set
  4.2.4 TPRO Produced notice
  4.2.4 TPUB Publisher
  4.2.1 TRCK Track number/Position in set
  4.2.4 TRSN Internet radio station name
  4.2.4 TRSO Internet radio station owner
  4.2.5 TSOA Album sort order
  4.2.5 TSOP Performer sort order
  4.2.5 TSOT Title sort order
  4.2.1 TSRC ISRC (international standard recording code)
  4.2.5 TSSE Software/Hardware and settings used for encoding
  4.2.1 TSST Set subtitle
  4.2.2 TXXX User defined text information frame

  4.1   UFID Unique file identifier
  4.22  USER Terms of use
  4.8   USLT Unsynchronised lyric/text transcription

  4.3.1 WCOM Commercial information
  4.3.1 WCOP Copyright/Legal information
  4.3.1 WOAF Official audio file webpage
  4.3.1 WOAR Official artist/performer webpage
  4.3.1 WOAS Official audio source webpage
  4.3.1 WORS Official Internet radio station homepage
  4.3.1 WPAY Payment
  4.3.1 WPUB Publishers official webpage
  4.3.2 WXXX User defined URL link frame
`

// Taken from http://id3.org/id3v2.3.0 §4.
const v23Spec = `
4.20    AENC    [[#sec4.20|Audio encryption]]
4.15    APIC    [#sec4.15 Attached picture]
4.11    COMM    [#sec4.11 Comments]
4.25    COMR    [#sec4.25 Commercial frame]
4.26    ENCR    [#sec4.26 Encryption method registration]
4.13    EQUA    [#sec4.13 Equalization]
4.6     ETCO    [#sec4.6 Event timing codes]
4.16    GEOB    [#sec4.16 General encapsulated object]
4.27    GRID    [#sec4.27 Group identification registration]
4.4     IPLS    [#sec4.4 Involved people list]
4.21    LINK    [#sec4.21 Linked information]
4.5     MCDI    [#sec4.5 Music CD identifier]
4.7     MLLT    [#sec4.7 MPEG location lookup table]
4.24    OWNE    [#sec4.24 Ownership frame]
4.28    PRIV    [#sec4.28 Private frame]
4.17    PCNT    [#sec4.17 Play counter]
4.18    POPM    [#sec4.18 Popularimeter]
4.22    POSS    [#sec4.22 Position synchronisation frame]
4.19    RBUF    [#sec4.19 Recommended buffer size]
4.12    RVAD    [#sec4.12 Relative volume adjustment]
4.14    RVRB    [#sec4.14 Reverb]
4.10    SYLT    [#sec4.10 Synchronized lyric/text]
4.8     SYTC    [#sec4.8 Synchronized tempo codes]
4.2.1   TALB    [#TALB Album/Movie/Show title]
4.2.1   TBPM    [#TBPM BPM (beats per minute)]
4.2.1   TCOM    [#TCOM Composer]
4.2.1   TCON    [#TCON Content type]
4.2.1   TCOP    [#TCOP Copyright message]
4.2.1   TDAT    [#TDAT Date]
4.2.1   TDLY    [#TDLY Playlist delay]
4.2.1   TENC    [#TENC Encoded by]
4.2.1   TEXT    [#TEXT Lyricist/Text writer]
4.2.1   TFLT    [#TFLT File type]
4.2.1   TIME    [#TIME Time]
4.2.1   TIT1    [#TIT1 Content group description]
4.2.1   TIT2    [#TIT2 Title/songname/content description]
4.2.1   TIT3    [#TIT3 Subtitle/Description refinement]
4.2.1   TKEY    [#TKEY Initial key]
4.2.1   TLAN    [#TLAN Language(s)]
4.2.1   TLEN    [#TLEN Length]
4.2.1   TMED    [#TMED Media type]
4.2.1   TOAL    [#TOAL Original album/movie/show title]
4.2.1   TOFN    [#TOFN Original filename]
4.2.1   TOLY    [#TOLY Original lyricist(s)/text writer(s)]
4.2.1   TOPE    [#TOPE Original artist(s)/performer(s)]
4.2.1   TORY    [#TORY Original release year]
4.2.1   TOWN    [#TOWN File owner/licensee]
4.2.1   TPE1    [#TPE1 Lead performer(s)/Soloist(s)]
4.2.1   TPE2    [#TPE2 Band/orchestra/accompaniment]
4.2.1   TPE3    [#TPE3 Conductor/performer refinement]
4.2.1   TPE4    [#TPE4 Interpreted, remixed, or otherwise modified by]
4.2.1   TPOS    [#TPOS Part of a set]
4.2.1   TPUB    [#TPUB Publisher]
4.2.1   TRCK    [#TRCK Track number/Position in set]
4.2.1   TRDA    [#TRDA Recording dates]
4.2.1   TRSN    [#TRSN Internet radio station name]
4.2.1   TRSO    [#TRSO Internet radio station owner]
4.2.1   TSIZ    [#TSIZ Size]
4.2.1   TSRC    [#TSRC ISRC (international standard recording code)]
4.2.1   TSSE    [#TSEE Software/Hardware and settings used for encoding]
4.2.1   TYER    [#TYER Year]
4.2.2   TXXX    [#TXXX User defined text information frame]
4.1     UFID    [#sec4.1 Unique file identifier]
4.23    USER    [#sec4.23 Terms of use]
4.9     USLT    [#sec4.9 Unsychronized lyric/text transcription]
4.3.1   WCOM    [#WCOM Commercial information]
4.3.1   WCOP    [#WCOP Copyright/Legal information]
4.3.1   WOAF    [#WOAF Official audio file webpage]
4.3.1   WOAR    [#WOAR Official artist/performer webpage]
4.3.1   WOAS    [#WOAS Official audio source webpage]
4.3.1   WORS    [#WORS Official internet radio station homepage]
4.3.1   WPAY    [#WPAY Payment]
4.3.1   WPUB    [#WPUB Publishers official webpage]
4.3.2   WXXX    [#WXXX User defined URL link frame]
`

var tmpl = template.Must(template.New("").Parse(
	"// Code generated by `go run generate_ids.go`. DO NOT EDIT." + `

// Copyright 2017 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a Modified
// BSD License that can be found in the LICENSE file.

package id3v2

// These are the standard frame ids as specified in the
// v2.4.0 and v2.3.0 specifications.
const (
{{- range .}}
	Frame{{.ID}} FrameID = '{{index .ID 0 | printf "%c"}}'<<24 | '{{index .ID 1 | printf "%c"}}'<<16 | '{{index .ID 2 | printf "%c"}}'<<8 | '{{index .ID 3 | printf "%c"}}' // {{.Description}}
{{- end}}
)

func (id FrameID) String() string {
	switch id {
{{- range .}}
	case Frame{{.ID}}:
		return "{{.ID}}: {{.Description}}"
{{- end}}
	default:
		buf := [4]byte{
			byte(id >> 24),
			byte(id >> 16),
			byte(id >> 8),
			byte(id),
		}
		return "FrameID(\"" + string(buf[:]) + "\")"
	}
}
`))

type frameID struct {
	ID, Description string
}

func main() {
	out := flag.String("out", "frame_ids.go", "the file to write the ids to")

	flag.Parse()

	var ids []frameID

	s := bufio.NewScanner(strings.NewReader(v24Spec))

	for s.Scan() {
		parts := strings.Fields(s.Text())
		if len(parts) < 2 || parts[0][:2] != "4." {
			continue
		}

		ids = append(ids, frameID{parts[1], strings.Join(parts[2:], " ")})
	}

	if s.Err() != nil {
		panic(s.Err())
	}

	s = bufio.NewScanner(strings.NewReader(v23Spec))

	for s.Scan() {
		parts := strings.Fields(s.Text())
		if len(parts) < 2 || parts[0][:2] != "4." {
			continue
		}

		comm := strings.IndexByte(s.Text(), '[')
		spce := strings.IndexByte(s.Text()[comm:], ' ')
		ids = append(ids, frameID{
			parts[1],
			strings.TrimSpace(s.Text()[comm+spce : len(s.Text())-1]),
		})
	}

	if s.Err() != nil {
		panic(s.Err())
	}

	uniq := ids[:0]
outer:
	for _, id := range ids {
		for _, iid := range uniq {
			if iid.ID == id.ID {
				continue outer
			}
		}

		uniq = append(uniq, id)
	}

	w, err := os.Create(*out)
	if err != nil {
		panic(err)
	}
	defer w.Close()

	if err := tmpl.Execute(w, uniq); err != nil {
		panic(err)
	}
}

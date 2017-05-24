// Copyright 2017 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a Modified
// BSD License that can be found in the LICENSE file.

// Package id3v2 implements support for reading ID3v2 tags.
package id3v2

//go:generate go run generate_ids.go

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"unicode/utf16"
)

// This is an implementation of v2.4.0 of the ID3v2 tagging format,
// defined in: http://id3.org/id3v2.4.0-structure, and v2.3.0 of
// the ID3v2 tagging format, defined in: http://id3.org/id3v2.3.0.

// Version is the version of the ID3v2 tag block.
type Version byte

const (
	// Version24 is v2.4.x of the ID3v2 specification.
	Version24 Version = 0x04
	// Version23 is v2.3.x of the ID3v2 specification.
	Version23 Version = 0x03
)

const (
	tagFlagUnsynchronisation = 1 << (7 - iota)
	tagFlagExtendedHeader
	tagFlagExperimental
	tagFlagFooter

	knownTagFlags = tagFlagUnsynchronisation | tagFlagExtendedHeader |
		tagFlagExperimental | tagFlagFooter
)

// FrameFlags are the frame-level ID3v2 flags.
type FrameFlags uint16

// These are the frame-level flags from v2.4.0 of the specification.
const (
	_ FrameFlags = 1 << (15 - iota)
	FrameFlagV24TagAlterPreservation
	FrameFlagV24FileAlterPreservation
	FrameFlagV24ReadOnly
	_
	_
	_
	_
	_
	FrameFlagV24GroupingIdentity
	_
	_
	FrameFlagV24Compression
	FrameFlagV24Encryption
	FrameFlagV24Unsynchronisation
	FrameFlagV24DataLengthIndicator
)

// These are the frame-level flags from v2.3.0 of the specification.
const (
	FrameFlagV23TagAlterPreservation FrameFlags = 1 << (15 - iota)
	FrameFlagV23FileAlterPreservation
	FrameFlagV23ReadOnly
	_
	_
	_
	_
	_
	FrameFlagV23Compression
	FrameFlagV23Encryption
	FrameFlagV23GroupingIdentity
)

const encodingFrameFlags FrameFlags = 0x00ff

const (
	textEncodingISO88591 = 0x00
	textEncodingUTF16    = 0x01
	textEncodingUTF16BE  = 0x02
	textEncodingUTF8     = 0x03
)

// FrameID is a four-byte frame identifier.
type FrameID uint32

const syncsafeInvalid = ^uint32(0)

func syncsafe(data []byte) uint32 {
	_ = data[3]

	if data[0]&0x80 != 0 || data[1]&0x80 != 0 ||
		data[2]&0x80 != 0 || data[3]&0x80 != 0 {
		return syncsafeInvalid
	}

	return uint32(data[0])<<21 | uint32(data[1])<<14 |
		uint32(data[2])<<7 | uint32(data[3])
}

var id3Token = []byte("ID3")

func id3Split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	i := bytes.Index(data, id3Token)
	if i == -1 {
		if len(data) < 2 {
			return 0, nil, nil
		}

		return len(data) - 2, nil, nil
	}

	data = data[i:]
	if len(data) < 10 {
		if atEOF {
			return 0, nil, io.ErrUnexpectedEOF
		}

		return i, nil, nil
	}

	size := syncsafe(data[6:])

	if data[3] == 0xff || data[4] == 0xff || size == syncsafeInvalid {
		// Skipping when we find the string "ID3" in the file but
		// the remaining header is invalid is consistent with the
		// detection logic in §3.1. This also reduces the
		// likelihood of errors being caused by the byte sequence
		// "ID3" (49 44 33) occurring in the audio, but does not
		// eliminate the possibility of errors in this case.
		//
		// Quoting from §3.1 of id3v2.4.0-structure.txt:
		//   An ID3v2 tag can be detected with the following pattern:
		//     $49 44 33 yy yy xx zz zz zz zz
		//   Where yy is less than $FF, xx is the 'flags' byte and zz
		//   is less than $80.
		return i + 3, nil, nil
	}

	if Version(data[3]) > Version24 {
		// Quoting from §3.1 of id3v2.4.0-structure.txt:
		//   If software with ID3v2.4.0 and below support should
		//   encounter version five or higher it should simply
		//   ignore the whole tag.
		return i + 3, nil, nil
	}

	if Version(data[3]) < Version23 {
		// This package only supports v2.3.0 and v2.4.0, skip
		// versions bellow v2.3.0.
		return i + 3, nil, nil
	}

	if data[5]&^knownTagFlags != 0 {
		// Skip tag blocks that contain unknown flags.
		//
		// Quoting from §3.1 of id3v2.4.0-structure.txt:
		//   If one of these undefined flags are set, the tag might
		//   not be readable for a parser that does not know the
		//   flags function.
		return i + 3, nil, nil
	}

	if data[5]&tagFlagFooter == tagFlagFooter {
		size += 10
	}

	if len(data) < 10+int(size) {
		if atEOF {
			return 0, nil, io.ErrUnexpectedEOF
		}

		return i, nil, nil
	}

	return i + 10 + int(size), data[:10+size], nil
}

const invalidFrameID = ^FrameID(0)

func validIDByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func frameID(data []byte) FrameID {
	_ = data[3]

	if validIDByte(data[0]) && validIDByte(data[1]) && validIDByte(data[2]) &&
		// Although it violates the specification, some software
		// incorrectly encodes v2.2.0 three character tags as
		// four character v2.3.0 tags with a trailing zero byte
		// when upgrading the tagging format version.
		(validIDByte(data[3]) || data[3] == 0) {
		return FrameID(binary.BigEndian.Uint32(data))
	}

	for _, v := range data {
		if v != 0 {
			return invalidFrameID
		}
	}

	// This is probably the begging of padding.
	return 0
}

var bufPool = &sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 4<<10)
		return &buf
	},
}

// Scan reads all valid ID3v2 tags from the reader and
// returns all the frames in order. It returns an error
// if the tags are invalid.
func Scan(r io.Reader) (Frames, error) {
	buf := bufPool.Get()
	defer bufPool.Put(buf)

	s := bufio.NewScanner(r)
	s.Buffer(*buf.(*[]byte), 20+1<<28)
	s.Split(id3Split)

	var frames Frames

	for s.Scan() {
		data := s.Bytes()

		header := data[:10]
		data = data[10:]

		if string(header[:3]) != "ID3" {
			panic("id3: bufio.Scanner failed")
		}

		version := Version(header[3])
		switch version {
		case Version24, Version23:
		default:
			panic("id3: bufio.Scanner failed")
		}

		flags := header[5]

		if flags&tagFlagFooter == tagFlagFooter {
			footer := data[len(data)-10:]
			data = data[:len(data)-10]

			if string(footer[:3]) != "3DI" ||
				!bytes.Equal(header[3:], footer[3:]) {
				return nil, errors.New("id3: invalid footer")
			}
		}

		if flags&tagFlagExtendedHeader == tagFlagExtendedHeader {
			var size uint32
			switch version {
			case Version24:
				size = syncsafe(data)
				if size == syncsafeInvalid {
					return nil, errors.New("id3: invalid extended header size")
				}
			case Version23:
				size = binary.BigEndian.Uint32(data) + 4
			default:
				panic("unhandled version")
			}

			if len(data) < int(size) {
				return nil, errors.New("id3: invalid extended header size")
			}

			extendedHeader := data[:size]
			data = data[size:]

			_ = extendedHeader
		}

	frames:
		for len(data) > 10 {
			_ = data[9]

			frame := &Frame{
				ID:      frameID(data),
				Version: version,
				Flags:   FrameFlags(binary.BigEndian.Uint16(data[8:])),
			}

			switch frame.ID {
			case 0:
				// We've probably hit padding, the padding
				// validity check below will handle this.
				break frames
			case invalidFrameID:
				return nil, errors.New("id3: invalid frame id")
			}

			var size uint32
			switch version {
			case Version24:
				size = syncsafe(data[4:])
				if size == syncsafeInvalid {
					return nil, errors.New("id3: invalid frame size")
				}
			case Version23:
				size = binary.BigEndian.Uint32(data[4:])
			default:
				panic("unhandled version")
			}

			if len(data) < 10+int(size) {
				return nil, errors.New("id3: frame size exceeds length of tag data")
			}

			if flags&tagFlagUnsynchronisation == tagFlagUnsynchronisation ||
				(version == Version24 && frame.Flags&FrameFlagV24Unsynchronisation != 0) {
				frame.Data = make([]byte, 0, size)

				for i := uint32(0); i < size; i++ {
					v := data[10+i]
					frame.Data = append(frame.Data, v)

					if v == 0xff && i+1 < size && data[10+i+1] == 0x00 {
						i++
					}
				}

				if version == Version24 {
					// Clear the frame level unsynchronisation flag
					frame.Flags &^= FrameFlagV24Unsynchronisation
				}
			} else {
				frame.Data = append([]byte(nil), data[10:10+size]...)
			}

			frames = append(frames, frame)
			data = data[10+size:]
		}

		if flags&tagFlagFooter == tagFlagFooter && len(data) != 0 {
			return nil, errors.New("id3: padding with footer")
		}

		for _, v := range data {
			if v != 0 {
				return nil, errors.New("id3: invalid padding")
			}
		}
	}

	if s.Err() != nil {
		return nil, s.Err()
	}

	return frames, nil
}

// Frames is a slice of ID3v2 frames.
type Frames []*Frame

// Lookup returns the last frame associated with a
// given frame id, or nil.
func (f Frames) Lookup(id FrameID) *Frame {
	for i := len(f) - 1; i >= 0; i-- {
		if f[i].ID == id {
			return f[i]
		}
	}

	return nil
}

// Frame is a single ID3v2 frame.
type Frame struct {
	ID      FrameID
	Version Version
	Flags   FrameFlags
	Data    []byte
}

func (f *Frame) String() string {
	version := "?"
	switch f.Version {
	case Version24:
		version = "v2.4"
	case Version23:
		version = "v2.3"
	}

	data, terminus := f.Data, ""
	if len(data) > 128 {
		data, terminus = data[:128], "..."
	}

	return fmt.Sprintf("&Frame{ID: %s, Version: %s, Flags: 0x%04x, Data: %d:%q%s}",
		f.ID.String(), version, f.Flags, len(f.Data), data, terminus)
}

// Text interprets the frame data as a text string,
// according to §4 of id3v2.4.0-structure.txt.
func (f *Frame) Text() (string, error) {
	if len(f.Data) == 0 {
		return "", errors.New("id3: frame data is invalid")
	}

	if f.Flags&encodingFrameFlags != 0 {
		return "", errors.New("id3: encoding frame flags are not supported")
	}

	data := f.Data[1:]
	var ord binary.ByteOrder = binary.BigEndian

	switch f.Data[0] {
	case textEncodingISO88591:
		for _, v := range data {
			if v&0x80 == 0 {
				continue
			}

			runes := make([]rune, len(data))
			for i, v := range data {
				runes[i] = rune(v)
			}

			if runes[len(runes)-1] == 0x00 {
				// The specification requires that the string be
				// terminated with 0x00, but not all implementations
				// do this.
				runes = runes[:len(runes)-1]
			}

			return string(runes), nil
		}

		fallthrough
	case textEncodingUTF8:
		if len(data) != 0 && data[len(data)-1] == 0x00 {
			// The specification requires that the string be
			// terminated with 0x00, but not all implementations
			// do this.
			data = data[:len(data)-1]
		}

		return string(data), nil
	case textEncodingUTF16:
		if len(data) < 2 {
			return "", errors.New("id3: missing UTF-16 BOM")
		}

		if data[0] == 0xff && data[1] == 0xfe {
			ord = binary.LittleEndian
		} else if data[0] == 0xfe && data[1] == 0xff {
			ord = binary.BigEndian
		} else {
			return "", errors.New("id3: invalid UTF-16 BOM")
		}

		data = data[2:]
		fallthrough
	case textEncodingUTF16BE:
		if len(data)%2 != 0 {
			return "", errors.New("id3: UTF-16 data is not even number of bytes")
		}

		u16s := make([]uint16, len(data)/2)
		for i := range u16s {
			u16s[i] = ord.Uint16(data[i*2:])
		}

		if len(u16s) != 0 && u16s[len(u16s)-1] == 0x0000 {
			// The specification requires that the string be
			// terminated with 0x00 0x00, but not all
			// implementations do this.
			u16s = u16s[:len(u16s)-1]
		}

		return string(utf16.Decode(u16s)), nil
	default:
		return "", errors.New("id3: frame uses unsupported encoding")
	}
}

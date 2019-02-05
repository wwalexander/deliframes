package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

const keyframeFlag int32 = 0x10

type header struct {
	ID   [4]byte
	Size int32
}

type indexEntry struct {
	ID     [4]byte
	Flags  int32
	Offset int32
	Size   int32
}

func removeKeyframes(dst io.WriteSeeker, src io.ReadSeeker) error {
	var head header
	if err := binary.Read(src, binary.LittleEndian, &head); err != nil {
		return err
	}
	if string(head.ID[:]) != "RIFF" {
		return errors.New("incorrect FOURCC")
	}
	var fileType [4]byte
	if _, err := src.Read(fileType[:]); err != nil {
		return err
	}
	if string(fileType[:]) != "AVI " {
		return errors.New("incorrect fileType")
	}
	var moviOffset int64
	var idxHeader *header
	for idxHeader == nil {
		var head header
		if err := binary.Read(src, binary.LittleEndian, &head); err != nil {
			return err
		}
		if string(head.ID[:]) == "LIST" {
			var listType [4]byte
			if _, err := src.Read(listType[:]); err != nil {
				return err
			}
			offset, err := src.Seek(-4, io.SeekCurrent)
			if err != nil {
				return err
			}
			if string(listType[:]) == "movi" {
				moviOffset = offset
			}
		}
		if string(head.ID[:]) == "idx1" {
			idxHeader = &head
		} else if _, err := src.Seek(int64(head.Size), io.SeekCurrent); err != nil {
			return err
		}
	}
	if moviOffset == 0 {
		return errors.New("missing movi")
	}
	end, err := src.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	end += int64(idxHeader.Size)
	if err != nil {
		return err
	}
	var iframeEntries []indexEntry
	for {
		offset, err := src.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		if offset >= end {
			break
		}
		var entry indexEntry
		if err := binary.Read(src, binary.LittleEndian, &entry); err != nil {
			return err
		}
		if entry.Flags&keyframeFlag != 0 {
			iframeEntries = append(iframeEntries, entry)
		}
	}
	for _, entry := range iframeEntries[1:] {
		if _, err := src.Seek(int64(entry.Offset), io.SeekStart); err != nil {
			return err
		}
		var head header
		if err := binary.Read(src, binary.LittleEndian, &head); err != nil {
			return err
		}
		if head.ID != entry.ID {
			if _, err := src.Seek(int64(entry.Offset)+moviOffset, io.SeekStart); err != nil {
				return err
			}
			if err := binary.Read(src, binary.LittleEndian, &head); err != nil {
				return err
			}
			if head.ID != entry.ID {
				return errors.New("incorrect index entry")
			}
			if _, err := src.Seek(-4, io.SeekCurrent); err != nil {
				return err
			}
		}
		offset, err := src.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		if _, err := dst.Seek(offset, io.SeekStart); err != nil {
			return err
		}
		if _, err := dst.Write([]byte("JUNK")); err != nil {
			return err
		}
	}
	return nil
}

const usage = `usage: deliframes source target

deliframes copies AVI video from source to target without keyframes.`

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(1)
	}
	src, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()
	dst, err := os.Create(flag.Arg(1))
	if err != nil {
		log.Fatal(err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		log.Fatal(err)
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		log.Fatal(err)
	}
	if err := removeKeyframes(dst, src); err != nil {
		log.Fatal(err)
	}
}

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

const flagKeyframe int32 = 0x10

type fourcc [4]byte

func (f fourcc) String() string {
	return string(f[:])
}

type header struct {
	ID     fourcc
	Size32 int32
}

func (h header) Size() int64 {
	return int64(h.Size32)
}

type index struct {
	ID       fourcc
	Flags    int32
	Offset32 int32
	Size     int32
}

func (idx index) Offset() int64 {
	return int64(idx.Offset32)
}

func removeKeyframes(avi io.ReadWriteSeeker) error {
	var head header
	if err := binary.Read(avi, binary.LittleEndian, &head); err != nil {
		return err
	}
	if head.ID.String() != "RIFF" {
		return errors.New("incorrect FOURCC")
	}
	var typ fourcc
	if _, err := avi.Read(typ[:]); err != nil {
		return err
	}
	if typ.String() != "AVI " {
		return errors.New("incorrect fileType")
	}
	var movi int64
	for {
		if err := binary.Read(
			avi,
			binary.LittleEndian,
			&head,
		); err != nil {
			return err
		}
		if head.ID.String() == "LIST" {
			var typ fourcc
			if _, err := avi.Read(typ[:]); err != nil {
				return err
			}
			offset, err := avi.Seek(-4, io.SeekCurrent)
			if err != nil {
				return err
			}
			if typ.String() == "movi" {
				movi = offset
			}
		}
		if head.ID.String() == "idx1" {
			break
		}
		if _, err := avi.Seek(head.Size(), io.SeekCurrent); err != nil {
			return err
		}
	}
	if movi == 0 {
		return errors.New("missing movi")
	}
	end, err := avi.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	end += head.Size()
	var keyframes []index
	for {
		offset, err := avi.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		if offset >= end {
			break
		}
		var idx index
		if err := binary.Read(
			avi,
			binary.LittleEndian,
			&idx,
		); err != nil {
			return err
		}
		if idx.Flags&flagKeyframe != 0 {
			keyframes = append(keyframes, idx)
		}
	}
	for _, idx := range keyframes[1:] {
		if _, err := avi.Seek(idx.Offset(), io.SeekStart); err != nil {
			return err
		}
		if err := binary.Read(
			avi,
			binary.LittleEndian,
			&head,
		); err != nil {
			return err
		}
		if head.ID != idx.ID {
			if _, err := avi.Seek(
				idx.Offset()+movi,
				io.SeekStart,
			); err != nil {
				return err
			}
			if err := binary.Read(
				avi,
				binary.LittleEndian,
				&head,
			); err != nil {
				return err
			}
			if head.ID != idx.ID {
				return errors.New("incorrect index entry")
			}
			if _, err := avi.Seek(-4, io.SeekCurrent); err != nil {
				return err
			}
		}
		if _, err := avi.Write([]byte("JUNK")); err != nil {
			return err
		}
	}
	return nil
}

const usage = `usage: deliframes file

deliframes removes AVI keyframes from file.`

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	avi, err := os.OpenFile(flag.Arg(0), os.O_RDWR, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer avi.Close()
	if err := removeKeyframes(avi); err != nil {
		log.Fatal(err)
	}
}

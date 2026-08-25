package singlestep

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

type ramEntry [2]uint32

// MOO binary test format reader (https://github.com/dbalsom/moo).

var regs16Names = []string{"ax", "bx", "cx", "dx", "cs", "ss", "ds", "es", "sp", "bp", "si", "di", "ip", "flags"}
var regs32Names = []string{"cr0", "cr3", "eax", "ebx", "ecx", "edx", "esi", "edi", "ebp", "esp", "cs", "ds", "es", "fs", "gs", "ss", "eip", "eflags", "dr6", "dr7"}

type mooState struct {
	Regs map[string]uint32
	Mask map[string]uint32 // undefined-bit masks (final only)
	RAM  []ramEntry
}

type mooTest struct {
	Idx       int
	Name      string
	Bytes     []byte
	Initial   mooState
	Final     mooState
	Exception *struct {
		Number   uint8
		FlagAddr uint32
	}
	Hash string
}

type mooFile struct {
	CPU   string
	Tests []mooTest
	Mask  map[string]uint32 // top-level masks
}

func readChunk(r []byte) (id string, payload []byte, rest []byte, err error) {
	if len(r) < 8 {
		return "", nil, nil, io.ErrUnexpectedEOF
	}
	id = string(r[:4])
	n := binary.LittleEndian.Uint32(r[4:8])
	if uint32(len(r)-8) < n {
		return "", nil, nil, fmt.Errorf("chunk %q truncated", id)
	}
	return id, r[8 : 8+n], r[8+n:], nil
}

func parseRegs16(p []byte) map[string]uint32 {
	m := map[string]uint32{}
	mask := binary.LittleEndian.Uint16(p)
	p = p[2:]
	for i, n := range regs16Names {
		if mask&(1<<uint(i)) != 0 {
			m[n] = uint32(binary.LittleEndian.Uint16(p))
			p = p[2:]
		}
	}
	return m
}

func parseRegs32(p []byte) map[string]uint32 {
	m := map[string]uint32{}
	mask := binary.LittleEndian.Uint32(p)
	p = p[4:]
	for i, n := range regs32Names {
		if mask&(1<<uint(i)) != 0 {
			m[n] = binary.LittleEndian.Uint32(p)
			p = p[4:]
		}
	}
	return m
}

func parseRAM(p []byte) []ramEntry {
	n := binary.LittleEndian.Uint32(p)
	p = p[4:]
	out := make([]ramEntry, 0, n)
	for i := uint32(0); i < n; i++ {
		out = append(out, ramEntry{binary.LittleEndian.Uint32(p), uint32(p[4])})
		p = p[5:]
	}
	return out
}

func parseState(p []byte) (mooState, error) {
	st := mooState{}
	for len(p) > 0 {
		id, payload, rest, err := readChunk(p)
		if err != nil {
			return st, err
		}
		switch id {
		case "REGS":
			st.Regs = parseRegs16(payload)
		case "RG32":
			st.Regs = parseRegs32(payload)
		case "RMSK":
			st.Mask = parseRegs16(payload)
		case "RM32":
			st.Mask = parseRegs32(payload)
		case "RAM ":
			st.RAM = parseRAM(payload)
		}
		p = rest
	}
	return st, nil
}

func loadMOO(path string) (*mooFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	mf := &mooFile{}
	p := data
	for len(p) > 0 {
		id, payload, rest, err := readChunk(p)
		if err != nil {
			return nil, err
		}
		p = rest
		switch id {
		case "MOO ":
			if len(payload) >= 12 {
				mf.CPU = strings.TrimSpace(string(payload[8:12]))
			}
		case "RMSK":
			mf.Mask = parseRegs16(payload)
		case "RM32":
			mf.Mask = parseRegs32(payload)
		case "TEST":
			t := mooTest{Idx: int(binary.LittleEndian.Uint32(payload))}
			q := payload[4:]
			for len(q) > 0 {
				sid, sp, srest, err := readChunk(q)
				if err != nil {
					return nil, err
				}
				q = srest
				switch sid {
				case "NAME":
					n := binary.LittleEndian.Uint32(sp)
					t.Name = string(sp[4 : 4+n])
				case "BYTS":
					n := binary.LittleEndian.Uint32(sp)
					t.Bytes = append([]byte(nil), sp[4:4+n]...)
				case "INIT":
					t.Initial, err = parseState(sp)
				case "FINA":
					t.Final, err = parseState(sp)
				case "EXCP":
					t.Exception = &struct {
						Number   uint8
						FlagAddr uint32
					}{sp[0], binary.LittleEndian.Uint32(sp[1:5])}
				case "HASH":
					t.Hash = fmt.Sprintf("%x", sp)
				}
				if err != nil {
					return nil, err
				}
			}
			mf.Tests = append(mf.Tests, t)
		}
	}
	return mf, nil
}

// Package singlestep runs the hardware-generated SingleStepTests suites
// (https://github.com/SingleStepTests) against the CPU core.
//
// Set SINGLESTEP_8088_DIR to the root of a checkout of SingleStepTests/8088
// (containing v2/ and v2/metadata.json). If unset, $XDG_CACHE_HOME/asm-emu/
// singlestep/8088 (default ~/.cache/...) is tried; otherwise the test skips.
package singlestep

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"assembly-emulator/emulator"
)

type cpuState struct {
	Regs  map[string]uint32 `json:"regs"`
	RAM   []ramEntry        `json:"ram"`
	Queue []byte            `json:"queue"`
}

type testCase struct {
	Idx     int      `json:"idx"`
	Name    string   `json:"name"`
	Bytes   []byte   `json:"bytes"`
	Initial cpuState `json:"initial"`
	Final   cpuState `json:"final"`
}

type opMeta struct {
	Status    string             `json:"status"`
	Flags     string             `json:"flags"`
	FlagsMask *uint32            `json:"flags-mask"`
	Reg       map[string]*opMeta `json:"reg"`
}

type metadata struct {
	Opcodes map[string]*opMeta `json:"opcodes"`
}

func suiteDir(env, name string) string {
	if d := os.Getenv(env); d != "" {
		return d
	}
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		home, _ := os.UserHomeDir()
		cache = filepath.Join(home, ".cache")
	}
	return filepath.Join(cache, "asm-emu", "singlestep", name)
}

func loadMeta(path string) (*metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var m metadata
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func loadTests(path string) ([]testCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r = f
	var tests []testCase
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		err = json.NewDecoder(gz).Decode(&tests)
		return tests, err
	}
	err = json.NewDecoder(r).Decode(&tests)
	return tests, err
}

// excluded8088 lists opcode files we do not run, with reasons.
var excluded8088 = map[string]string{
	"F4": "HLT: test ends in halted state with bus-level semantics",
	"9B": "WAIT: no coprocessor",
	"D8": "FPU escape", "D9": "FPU escape", "DA": "FPU escape", "DB": "FPU escape",
	"DC": "FPU escape", "DD": "FPU escape", "DE": "FPU escape", "DF": "FPU escape",
}

var regNames8088 = []string{"ax", "bx", "cx", "dx", "cs", "ss", "ds", "es", "sp", "bp", "si", "di", "ip", "flags"}

func setReg(c *emulator.CPU, name string, v uint32) {
	switch name {
	case "ax":
		c.SetAX(uint16(v))
	case "bx":
		c.SetBX(uint16(v))
	case "cx":
		c.SetCX(uint16(v))
	case "dx":
		c.SetDX(uint16(v))
	case "sp":
		c.SetSP(uint16(v))
	case "bp":
		c.SetBP(uint16(v))
	case "si":
		c.SetSI(uint16(v))
	case "di":
		c.SetDI(uint16(v))
	case "cs":
		c.Segs[emulator.SegCS] = uint16(v)
	case "ds":
		c.Segs[emulator.SegDS] = uint16(v)
	case "es":
		c.Segs[emulator.SegES] = uint16(v)
	case "ss":
		c.Segs[emulator.SegSS] = uint16(v)
	case "ip":
		c.EIP = v & 0xFFFF
	case "flags":
		c.Flags = v
	}
}

func getReg(c *emulator.CPU, name string) uint32 {
	switch name {
	case "ax":
		return uint32(c.AX())
	case "bx":
		return uint32(c.BX())
	case "cx":
		return uint32(c.CX())
	case "dx":
		return uint32(c.DX())
	case "sp":
		return uint32(c.SP())
	case "bp":
		return uint32(c.BP())
	case "si":
		return uint32(c.SI())
	case "di":
		return uint32(c.DI())
	case "cs":
		return uint32(c.Segs[emulator.SegCS])
	case "ds":
		return uint32(c.Segs[emulator.SegDS])
	case "es":
		return uint32(c.Segs[emulator.SegES])
	case "ss":
		return uint32(c.Segs[emulator.SegSS])
	case "ip":
		return c.EIP & 0xFFFF
	case "flags":
		return c.Flags & 0xFFFF
	}
	return 0
}

// opcodeKey extracts "XX" or "XX.R" from a file name like "80.7.json.gz".
func opcodeKey(name string) (op string, reg string) {
	base := name
	for _, suf := range []string{".json.gz", ".json"} {
		base = strings.TrimSuffix(base, suf)
	}
	parts := strings.SplitN(base, ".", 2)
	if len(parts) == 2 {
		return strings.ToUpper(parts[0]), parts[1]
	}
	return strings.ToUpper(parts[0]), ""
}

func TestSingleStep8088(t *testing.T) {
	dir := suiteDir("SINGLESTEP_8088_DIR", "8088")
	v2 := filepath.Join(dir, "v2")
	meta, err := loadMeta(filepath.Join(v2, "metadata.json"))
	if err != nil {
		t.Skipf("SingleStepTests/8088 not available at %s: %v", dir, err)
	}
	files, _ := filepath.Glob(filepath.Join(v2, "*.json*"))
	if len(files) == 0 {
		t.Skipf("no test files in %s", v2)
	}
	sort.Strings(files)
	limit := sampleLimit()

	for _, file := range files {
		if strings.HasSuffix(file, "metadata.json") {
			continue
		}
		op, reg := opcodeKey(filepath.Base(file))
		name := op
		if reg != "" {
			name = op + "." + reg
		}
		file := file
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if why, ok := excluded8088[op]; ok {
				t.Skip(why)
			}
			m := meta.Opcodes[op]
			if m == nil {
				t.Skipf("no metadata for %s", op)
			}
			if reg != "" && m.Reg != nil {
				if rm := m.Reg[reg]; rm != nil {
					if rm.Status != "" {
						m = &opMeta{Status: rm.Status, Flags: rm.Flags, FlagsMask: rm.FlagsMask}
					}
				}
			}
			switch m.Status {
			case "normal", "alias", "undocumented":
			default:
				t.Skipf("status %q", m.Status)
			}
			mask := uint32(0xFFFF)
			if m.FlagsMask != nil {
				mask = *m.FlagsMask
			}
			tests, err := loadTests(file)
			if err != nil {
				t.Fatal(err)
			}
			if limit > 0 && len(tests) > limit {
				tests = tests[:limit]
			}
			fails := 0
			dump := os.Getenv("SINGLESTEP_DUMP") != ""
			for _, tc := range tests {
				if msg := runOne(tc, mask); msg != "" {
					fails++
					if fails <= 5 {
						t.Errorf("#%d %s: %s", tc.Idx, tc.Name, msg)
						if dump {
							t.Logf("initial: %v final: %v", tc.Initial.Regs, tc.Final.Regs)
						}
					}
				}
			}
			if fails > 0 {
				t.Errorf("%d/%d failed", fails, len(tests))
			}
		})
	}
}

func runOne(tc testCase, mask uint32) string {
	c := emulator.NewCPU(emulator.Model8088)
	for _, n := range regNames8088 {
		if v, ok := tc.Initial.Regs[n]; ok {
			setReg(c, n, v)
		}
	}
	for _, e := range tc.Initial.RAM {
		c.Mem.RAM[e[0]&0xFFFFF] = byte(e[1])
	}
	// Execute a single instruction. Errors are emulator faults.
	// 8088 tests capture the state right after the instruction (no HLT).
	if err := c.Step(); err != nil {
		return fmt.Sprintf("fault: %v", err)
	}
	var problems []string
	for _, n := range regNames8088 {
		want, ok := tc.Final.Regs[n]
		if !ok {
			want = tc.Initial.Regs[n]
		}
		got := getReg(c, n)
		if n == "flags" {
			want &= mask
			got &= mask
		}
		if got != want {
			problems = append(problems, fmt.Sprintf("%s=%04X want %04X", n, got, want))
		}
	}
	// On a divide fault the flags pushed on the stack contain undefined
	// bits; mask them the same way as the flags register.
	var flagAddr uint32 = 0xFFFFFFFF
	if c.LastVector == 0 {
		flagAddr = (uint32(c.Segs[emulator.SegSS])<<4 + uint32(uint16(c.SP()+4))) & 0xFFFFF
	}
	for _, e := range tc.Final.RAM {
		got := uint32(c.Mem.RAM[e[0]&0xFFFFF])
		want := e[1]
		if e[0] == flagAddr {
			got &= mask & 0xFF
			want &= mask & 0xFF
		} else if e[0] == flagAddr+1 {
			got &= mask >> 8
			want &= mask >> 8
		}
		if got != want {
			problems = append(problems, fmt.Sprintf("[%05X]=%02X want %02X", e[0], got, e[1]))
		}
	}
	if len(problems) > 0 {
		return strings.Join(problems, ", ") + fmt.Sprintf(" (bytes % X)", tc.Bytes)
	}
	return ""
}

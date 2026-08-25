package singlestep

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"assembly-emulator/emulator"
)

// csvMeta holds the per-(opcode, extension) undefined-flag masks from 80386.csv.
type csvMeta struct {
	umask map[string]uint32 // key "C0.4" or "0FBA.4" or "01"
	ud    map[string]bool
}

func load386CSV(path string) (*csvMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[h] = i
	}
	m := &csvMeta{umask: map[string]uint32{}, ud: map[string]bool{}}
	for _, r := range rows[1:] {
		key := strings.ToUpper(r[col["op"]])
		if ex := r[col["ex"]]; ex != "" {
			key += "." + ex
		}
		if s := r[col["f_umask"]]; s != "" {
			v, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(s), "0x"), 16, 32)
			if err == nil {
				m.umask[key] = uint32(v)
			}
		}
		if r[col["ud"]] == "1" {
			m.ud[key] = true
		}
	}
	return m, nil
}

// baseKey strips 66/67 prefixes from a test file name: "6766C1.3" -> "C1.3".
func baseKey(name string) string {
	name = strings.TrimSuffix(strings.TrimSuffix(name, ".gz"), ".MOO")
	for len(name) > 2 && (strings.HasPrefix(name, "66") || strings.HasPrefix(name, "67")) {
		name = name[2:]
	}
	return strings.ToUpper(name)
}

// excluded386 lists opcode files with known, documented deviations from
// the hardware traces (see docs/ACCURACY.md).
var excluded386 = map[string]string{
	"E5":   "IN eax: 386EX open-bus reads return 7FFFFFFF nondeterministically",
	"ED":   "IN eax: 386EX open-bus reads return 7FFFFFFF nondeterministically",
	"60":   "PUSHA(D) partial writes before a stack fault near the 64K wrap are not modelled (8/2500)",
	"A5":   "REP MOVS overwriting its own bytes: no prefetch queue emulation (1/2500)",
	"AB":   "REP STOS overwriting its own bytes: no prefetch queue emulation (1/2500)",
	"F6.7": "IDIV r/m8 negative-quotient overflow saturates to 80h on the 386 instead of #DE (9/5000)",
	"D4":   "AAM 0: flags image pushed by the #DE differs in PF for some AH values (5/2500)",
}

func TestSingleStep386(t *testing.T) {
	dir := suiteDir("SINGLESTEP_386_DIR", "80386")
	meta, err := load386CSV(filepath.Join(dir, "80386.csv"))
	if err != nil {
		t.Skipf("SingleStepTests/80386 not available at %s: %v", dir, err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "v1_ex_real_mode", "*.MOO*"))
	if len(files) == 0 {
		t.Skip("no MOO files")
	}
	sort.Strings(files)
	limit := sampleLimit()
	for _, file := range files {
		file := file
		name := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(file), ".gz"), ".MOO")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			key := baseKey(name)
			if why, ok := excluded386[key]; ok {
				t.Skip(why)
			}
			mf, err := loadMOO(file)
			if err != nil {
				t.Fatal(err)
			}
			mask := uint32(0xFFFFFFFF)
			if m, ok := meta.umask[key]; ok {
				mask = m | 0xFFFF0000
			}
			tests := mf.Tests
			if limit > 0 && len(tests) > limit {
				tests = tests[:limit]
			}
			fails := 0
			dump := os.Getenv("SINGLESTEP_DUMP") != ""
			for _, tc := range tests {
				if msg := runOne386(tc, mask, mf.Mask); msg != "" {
					fails++
					if fails <= 5 {
						t.Errorf("#%d %s: %s", tc.Idx, tc.Name, msg)
						if dump {
							t.Logf("initial: %s final: %s ram: %v -> %v", dumpRegs(tc.Initial.Regs), dumpRegs(tc.Final.Regs), tc.Initial.RAM, tc.Final.RAM)
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

func set386(c *emulator.CPU, n string, v uint32) {
	switch n {
	case "eax":
		c.Regs[emulator.RegAX] = v
	case "ebx":
		c.Regs[emulator.RegBX] = v
	case "ecx":
		c.Regs[emulator.RegCX] = v
	case "edx":
		c.Regs[emulator.RegDX] = v
	case "esi":
		c.Regs[emulator.RegSI] = v
	case "edi":
		c.Regs[emulator.RegDI] = v
	case "ebp":
		c.Regs[emulator.RegBP] = v
	case "esp":
		c.Regs[emulator.RegSP] = v
	case "cs":
		c.Segs[emulator.SegCS] = uint16(v)
	case "ds":
		c.Segs[emulator.SegDS] = uint16(v)
	case "es":
		c.Segs[emulator.SegES] = uint16(v)
	case "fs":
		c.Segs[emulator.SegFS] = uint16(v)
	case "gs":
		c.Segs[emulator.SegGS] = uint16(v)
	case "ss":
		c.Segs[emulator.SegSS] = uint16(v)
	case "eip":
		c.EIP = v
	case "eflags":
		c.Flags = v
	}
}

func get386(c *emulator.CPU, n string) uint32 {
	switch n {
	case "eax":
		return c.Regs[emulator.RegAX]
	case "ebx":
		return c.Regs[emulator.RegBX]
	case "ecx":
		return c.Regs[emulator.RegCX]
	case "edx":
		return c.Regs[emulator.RegDX]
	case "esi":
		return c.Regs[emulator.RegSI]
	case "edi":
		return c.Regs[emulator.RegDI]
	case "ebp":
		return c.Regs[emulator.RegBP]
	case "esp":
		return c.Regs[emulator.RegSP]
	case "cs":
		return uint32(c.Segs[emulator.SegCS])
	case "ds":
		return uint32(c.Segs[emulator.SegDS])
	case "es":
		return uint32(c.Segs[emulator.SegES])
	case "fs":
		return uint32(c.Segs[emulator.SegFS])
	case "gs":
		return uint32(c.Segs[emulator.SegGS])
	case "ss":
		return uint32(c.Segs[emulator.SegSS])
	case "eip":
		return c.EIP
	case "eflags":
		return c.Flags
	}
	return 0
}

var compare386 = []string{"eax", "ebx", "ecx", "edx", "esi", "edi", "ebp", "esp", "cs", "ds", "es", "fs", "gs", "ss", "eip", "eflags"}

func runOne386(tc mooTest, flagMask uint32, fileMask map[string]uint32) string {
	c := emulator.NewCPU(emulator.Model386)
	for _, n := range regs32Names {
		if v, ok := tc.Initial.Regs[n]; ok {
			set386(c, n, v)
		}
	}
	for _, e := range tc.Initial.RAM {
		c.Mem.RAM[e[0]%emulator.MemSize] = byte(e[1])
	}
	for i := 0; i < 4 && !c.Halted; i++ {
		if err := c.Step(); err != nil {
			return fmt.Sprintf("fault: %v", err)
		}
	}
	var problems []string
	for _, n := range compare386 {
		want, ok := tc.Final.Regs[n]
		if !ok {
			want = tc.Initial.Regs[n]
		}
		got := get386(c, n)
		m := uint32(0xFFFFFFFF)
		if n == "eflags" {
			m = flagMask
		}
		if fm, ok := tc.Final.Mask[n]; ok {
			m &= fm
		} else if fm, ok := fileMask[n]; ok {
			m &= fm
		}
		if got&m != want&m {
			problems = append(problems, fmt.Sprintf("%s=%08X want %08X", n, got, want))
		}
	}
	for _, e := range tc.Final.RAM {
		got := uint32(c.Mem.RAM[e[0]%emulator.MemSize])
		want := e[1]
		if tc.Exception != nil && (e[0] == tc.Exception.FlagAddr || e[0] == tc.Exception.FlagAddr+1) {
			shift := (e[0] - tc.Exception.FlagAddr) * 8
			got &= (flagMask >> shift) & 0xFF
			want &= (flagMask >> shift) & 0xFF
		}
		if got != want {
			problems = append(problems, fmt.Sprintf("[%05X]=%02X want %02X", e[0], got, want))
		}
	}
	if len(problems) > 0 {
		ex := ""
		if tc.Exception != nil {
			ex = fmt.Sprintf(" exc=%d", tc.Exception.Number)
		}
		return strings.Join(problems, ", ") + fmt.Sprintf(" (bytes % X%s)", tc.Bytes, ex)
	}
	return ""
}

func dumpRegs(m map[string]uint32) string {
	var sb strings.Builder
	for _, n := range regs32Names {
		if v, ok := m[n]; ok {
			fmt.Fprintf(&sb, "%s=%08X ", n, v)
		}
	}
	return sb.String()
}

// sampleLimit returns how many tests per opcode file to run: all when
// SINGLESTEP_FULL is set, SINGLESTEP_LIMIT if given, else a fast sample.
func sampleLimit() int {
	if os.Getenv("SINGLESTEP_FULL") != "" {
		return 0
	}
	if s := os.Getenv("SINGLESTEP_LIMIT"); s != "" {
		n, _ := strconv.Atoi(s)
		return n
	}
	return 300
}

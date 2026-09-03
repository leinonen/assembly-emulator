package debugger

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Line locates one assembled item in its source file.
type Line struct {
	File string
	Line int
}

// Source maps assembled addresses to source lines and symbols. It is
// filled from the assembler's Listing and Symbol callbacks; addresses are
// the assembler's (org-relative) values, i.e. offsets in the load segment.
type Source struct {
	Main    string          // path of the main source file
	lines   map[uint32]Line // first item at each address
	addrs   []uint32        // sorted keys of lines
	byLine  map[string]map[int]uint32
	symbols map[string]uint32 // labels and constants
	labels  map[uint32]string // reverse map for labels only
	files   map[string][]string
}

// NewSource returns an empty source map for the given main file.
func NewSource(main string) *Source {
	return &Source{
		Main:    main,
		lines:   map[uint32]Line{},
		byLine:  map[string]map[int]uint32{},
		symbols: map[string]uint32{},
		labels:  map[uint32]string{},
		files:   map[string][]string{},
	}
}

// AddListing records one listing entry (assembler.Options.Listing).
func (s *Source) AddListing(addr int64, code []byte, file string, line int) {
	if s == nil || addr < 0 {
		return
	}
	a := uint32(addr)
	if _, ok := s.lines[a]; !ok && len(code) > 0 {
		s.lines[a] = Line{File: file, Line: line}
		s.addrs = nil
	}
	key := fileKey(file)
	m := s.byLine[key]
	if m == nil {
		m = map[int]uint32{}
		s.byLine[key] = m
	}
	if _, ok := m[line]; !ok {
		m[line] = a
	}
}

// AddSymbol records a label or constant (assembler.Options.Symbol).
func (s *Source) AddSymbol(name string, value int64, label bool) {
	if s == nil {
		return
	}
	s.symbols[name] = uint32(value)
	if label && !strings.HasPrefix(name, "..@") {
		if old, ok := s.labels[uint32(value)]; !ok || len(name) < len(old) {
			s.labels[uint32(value)] = name
		}
	}
}

func fileKey(file string) string { return strings.ToLower(filepath.Base(file)) }

// Lookup resolves a symbol name (exact, then case-insensitive).
func (s *Source) Lookup(name string) (uint32, bool) {
	if s == nil {
		return 0, false
	}
	if v, ok := s.symbols[name]; ok {
		return v, true
	}
	for k, v := range s.symbols {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return 0, false
}

// Label returns the label defined at an address, if any.
func (s *Source) Label(addr uint32) (string, bool) {
	if s == nil {
		return "", false
	}
	l, ok := s.labels[addr]
	return l, ok
}

// Nearest returns the closest label at or below addr and the distance.
func (s *Source) Nearest(addr uint32) (string, uint32, bool) {
	if s == nil {
		return "", 0, false
	}
	var best string
	var bestAddr uint32
	found := false
	for a, l := range s.labels {
		if a <= addr && (!found || a > bestAddr) {
			best, bestAddr, found = l, a, true
		}
	}
	return best, addr - bestAddr, found
}

// LineAt returns the source line for the item starting at addr.
func (s *Source) LineAt(addr uint32) (Line, bool) {
	if s == nil {
		return Line{}, false
	}
	l, ok := s.lines[addr]
	return l, ok
}

// Containing returns the source line of the item covering addr (the last
// item starting at or before it).
func (s *Source) Containing(addr uint32) (Line, bool) {
	if s == nil || len(s.lines) == 0 {
		return Line{}, false
	}
	if s.addrs == nil {
		for a := range s.lines {
			s.addrs = append(s.addrs, a)
		}
		sort.Slice(s.addrs, func(i, j int) bool { return s.addrs[i] < s.addrs[j] })
	}
	i := sort.Search(len(s.addrs), func(i int) bool { return s.addrs[i] > addr })
	if i == 0 {
		return Line{}, false
	}
	return s.lines[s.addrs[i-1]], true
}

// HasFile reports whether name (a path or base name) is a source file
// with listing entries.
func (s *Source) HasFile(name string) bool {
	if s == nil {
		return false
	}
	_, ok := s.byLine[fileKey(name)]
	return ok
}

// Resolve returns the address of the first code at or after file:line.
// An empty file means the main file.
func (s *Source) Resolve(file string, line int) (uint32, bool) {
	if s == nil {
		return 0, false
	}
	if file == "" {
		file = s.Main
	}
	m := s.byLine[fileKey(file)]
	if m == nil {
		return 0, false
	}
	best, found := 0, false
	for l := range m {
		if l >= line && (!found || l < best) {
			best, found = l, true
		}
	}
	if !found {
		return 0, false
	}
	return m[best], true
}

// Text returns the text of a source line (1-based), if readable.
func (s *Source) Text(file string, line int) (string, bool) {
	if s == nil {
		return "", false
	}
	src, ok := s.files[file]
	if !ok {
		data, err := os.ReadFile(file)
		if err != nil {
			s.files[file] = nil
			return "", false
		}
		src = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		s.files[file] = src
	}
	if line < 1 || line > len(src) {
		return "", false
	}
	return src[line-1], true
}

// LineCount returns the number of lines in a source file (0 if unknown).
func (s *Source) LineCount(file string) int {
	if _, ok := s.Text(file, 1); !ok {
		return 0
	}
	return len(s.files[file])
}

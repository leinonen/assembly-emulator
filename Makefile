.PHONY: build clean test test-full examples gifs run help

BINARY=asm-emu
EXAMPLES=$(wildcard examples/*.asm)
GIF_EXAMPLES=bouncing-line cube fire plasma rotozoom sine_scroller starfield tunnel

build:
	go build -o $(BINARY) .

clean:
	rm -f $(BINARY) examples/*.com

# Unit tests, examples, corpus and a fast sample of the CPU suites.
test:
	go test ./...

# Complete SingleStepTests runs (needs ~/.cache/asm-emu/singlestep, ~10 min).
test-full:
	SINGLESTEP_FULL=1 go test ./tests/singlestep

# Assemble every example to a .COM next to it.
examples: build
	@for f in $(EXAMPLES); do ./$(BINARY) asm $$f -o $${f%.asm}.com || exit 1; done

# Regenerate the example GIFs deterministically.
gifs: build
	@for n in $(GIF_EXAMPLES); do ./$(BINARY) run -gif examples/gifs/$$n.gif -gif-frames 100 examples/$$n.asm; done

run: build
	@if [ -z "$(FILE)" ]; then echo "Usage: make run FILE=examples/plasma.asm"; exit 1; fi
	./$(BINARY) run $(FILE)

help:
	@echo "make build | test | test-full | examples | gifs | run FILE=..."

.DEFAULT_GOAL := build

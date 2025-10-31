.PHONY: build clean run test help

# Binary name
BINARY=asm-emu

# Build the emulator
build:
	@echo "Building assembly emulator..."
	go build -o $(BINARY)
	@echo "Build complete: ./$(BINARY)"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY)
	@echo "Clean complete"

# Run a test program
run:
	@if [ -z "$(FILE)" ]; then \
		echo "Usage: make run FILE=examples/pixels.asm"; \
		exit 1; \
	fi
	./$(BINARY) $(FILE)

# Run all working examples as tests
test: build
	@echo "Testing simple.asm (headless)..."
	@./$(BINARY) examples/simple.asm | grep -q "Program halted" && echo "✓ simple.asm works" || echo "✗ simple.asm failed"
	@echo ""
	@echo "Testing pixels.asm (graphics)..."
	@timeout 2 ./$(BINARY) examples/pixels.asm 2>&1 | grep -q "Mode 13h detected" && echo "✓ pixels.asm works" || echo "✗ pixels.asm failed"
	@pkill -9 asm-emu 2>/dev/null || true
	@echo ""
	@echo "All tests complete!"

# Install to system (optional)
install: build
	@echo "Installing to /usr/local/bin..."
	sudo cp $(BINARY) /usr/local/bin/
	@echo "Installed! You can now run: $(BINARY) <file.asm>"

# Uninstall from system
uninstall:
	@echo "Uninstalling from /usr/local/bin..."
	sudo rm -f /usr/local/bin/$(BINARY)
	@echo "Uninstalled"

# Show help
help:
	@echo "Assembly Emulator - Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  make build       - Build the emulator (default)"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make run FILE=   - Run a specific assembly file"
	@echo "  make test        - Run tests with example files"
	@echo "  make install     - Install to /usr/local/bin"
	@echo "  make uninstall   - Remove from /usr/local/bin"
	@echo "  make help        - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make"
	@echo "  make run FILE=examples/pixels.asm"
	@echo "  make test"

# Default target
.DEFAULT_GOAL := build

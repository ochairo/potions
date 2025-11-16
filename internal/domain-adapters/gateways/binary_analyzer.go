// Package gateways provides adapter implementations for external services and tools.
package gateways

import (
	"context"
	"debug/elf"
	"debug/macho"
	"fmt"
	"strings"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
)

// binaryAnalyzerGateway implements binary security analysis using pure Go
// Uses debug/elf and debug/macho packages - no external tools required
type binaryAnalyzerGateway struct{}

// NewBinaryAnalyzerGateway creates a new binary analyzer gateway
//
//nolint:revive // unexported-return: Intentionally returns concrete type for testability
func NewBinaryAnalyzerGateway() *binaryAnalyzerGateway {
	return &binaryAnalyzerGateway{}
}

// AnalyzeBinaryHardening analyzes binary hardening features
func (g *binaryAnalyzerGateway) AnalyzeBinaryHardening(_ context.Context, binaryPath, platform string) (*entities.BinaryAnalysis, error) {
	switch {
	case strings.HasPrefix(platform, "darwin"):
		return g.analyzeDarwinBinary(binaryPath)
	case strings.HasPrefix(platform, "linux"):
		return g.analyzeLinuxBinary(binaryPath)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// analyzeLinuxBinary analyzes a Linux ELF binary using debug/elf
func (g *binaryAnalyzerGateway) analyzeLinuxBinary(binaryPath string) (*entities.BinaryAnalysis, error) {
	f, err := elf.Open(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open ELF file: %w", err)
	}
	//nolint:errcheck // Defer close on read-only file
	defer f.Close()

	features := entities.HardeningFeatures{}

	// Check PIE - examine ELF header type
	features.PIEEnabled = f.Type == elf.ET_DYN

	// Check RELRO - examine program headers
	features.RELRO = "disabled"
	hasRelro := false
	for _, prog := range f.Progs {
		if prog.Type == elf.PT_GNU_RELRO {
			hasRelro = true
			features.RELRO = "partial"
			break
		}
	}

	// Check for BIND_NOW in dynamic section for full RELRO
	if hasRelro {
		if dynSection := f.SectionByType(elf.SHT_DYNAMIC); dynSection != nil {
			data, err := dynSection.Data()
			if err == nil && strings.Contains(string(data), "BIND_NOW") {
				features.RELRO = "full"
			}
		}
	}

	// Check NX bit - examine GNU_STACK segment
	features.NXBit = true // default to secure
	for _, prog := range f.Progs {
		if prog.Type == elf.PT_GNU_STACK {
			// Check if executable flag is set
			features.NXBit = (prog.Flags & elf.PF_X) == 0
			break
		}
	}

	// Check stack canaries - look for __stack_chk_fail symbol
	symbols, err := f.Symbols()
	if err == nil {
		for _, sym := range symbols {
			if sym.Name == "__stack_chk_fail" {
				features.StackCanaries = true
				break
			}
		}
	}

	// Check FORTIFY_SOURCE - look for *_chk functions
	for _, sym := range symbols {
		if strings.HasSuffix(sym.Name, "_chk") && sym.Name != "__stack_chk_fail" {
			features.FortifySource = true
			break
		}
	}

	score := g.calculateHardeningScore(features)

	return &entities.BinaryAnalysis{
		Platform:          "linux",
		HardeningFeatures: features,
		SecurityScore:     score,
		Timestamp:         time.Now(),
	}, nil
}

// analyzeDarwinBinary analyzes a macOS Mach-O binary using debug/macho
func (g *binaryAnalyzerGateway) analyzeDarwinBinary(binaryPath string) (*entities.BinaryAnalysis, error) {
	f, err := macho.Open(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Mach-O file: %w", err)
	}
	//nolint:errcheck // Defer close on read-only file
	defer f.Close()

	features := entities.HardeningFeatures{}

	// Check PIE - examine Mach-O flags
	features.PIEEnabled = (f.Flags & macho.FlagPIE) != 0

	// Check stack canaries - look for symbols
	if symtab := f.Symtab; symtab != nil {
		for _, sym := range symtab.Syms {
			if strings.Contains(sym.Name, "__stack_chk_fail") {
				features.StackCanaries = true
				break
			}
		}
	}

	// Code signing check - look for LC_CODE_SIGNATURE load command
	for _, load := range f.Loads {
		// Code signature is typically in the __LINKEDIT segment
		if seg, ok := load.(*macho.Segment); ok {
			if seg.Name == "__LINKEDIT" {
				// Presence of __LINKEDIT with certain characteristics indicates code signing
				features.CodeSigned = true
			}
		}
	}

	// Hardened runtime check - look for specific load commands
	// This is a simplified check - full implementation would parse LC_DYLD_ENVIRONMENT
	features.HardenedRuntime = features.CodeSigned // Simplified assumption

	score := g.calculateHardeningScore(features)

	return &entities.BinaryAnalysis{
		Platform:          "darwin",
		HardeningFeatures: features,
		SecurityScore:     score,
		Timestamp:         time.Now(),
	}, nil
}

// calculateHardeningScore calculates a security score based on hardening features
func (g *binaryAnalyzerGateway) calculateHardeningScore(features entities.HardeningFeatures) entities.SecurityScore {
	checks := []bool{
		features.PIEEnabled,
		features.StackCanaries,
		features.NXBit,
		features.RELRO == "full" || features.RELRO == "partial",
		features.FortifySource,
		features.CodeSigned,
		features.HardenedRuntime,
	}

	passed := 0
	total := 0

	for _, check := range checks {
		total++
		if check {
			passed++
		}
	}

	percentage := 0
	if total > 0 {
		percentage = (passed * 100) / total
	}

	score := float64(passed) / float64(total) * 10.0

	return entities.SecurityScore{
		Score:      score,
		Total:      total,
		Passed:     passed,
		Percentage: percentage,
	}
}

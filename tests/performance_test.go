package test

import (
	"documents-worker/media"
	"documents-worker/types"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func RunPerformanceTest() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run performance_test.go <test_file_path>")
		os.Exit(1)
	}

	testFile := os.Args[1]

	// Check if test file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		fmt.Printf("Test file not found: %s\n", testFile)
		os.Exit(1)
	}

	// Get file info
	info, err := os.Stat(testFile)
	if err != nil {
		fmt.Printf("Error getting file info: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🧪 Performance Test for File Processing\n")
	fmt.Printf("📄 File: %s\n", filepath.Base(testFile))
	fmt.Printf("📏 Size: %.2f MB\n", float64(info.Size())/(1024*1024))
	fmt.Printf("⏰ Testing started at: %s\n\n", time.Now().Format("15:04:05"))

	// Create test converter
	converter := &types.MediaConverter{
		Kind: types.ImageKind,
		Search: types.MediaSearch{
			Width:   intPtr(800),
			Height:  intPtr(600),
			Quality: intPtr(85),
		},
		Format: stringPtr("webp"),
	}

	// Test 1: Original processing
	fmt.Printf("🔄 Test 1: Original Processing\n")
	start := time.Now()

	outputFile1, err := media.ExecCommand(false, testFile, converter)
	duration1 := time.Since(start)

	if err != nil {
		fmt.Printf("❌ Original processing failed: %v\n", err)
	} else {
		fmt.Printf("✅ Original processing completed in: %v\n", duration1)
		if outputFile1 != nil {
			outputFile1.Close()
			os.Remove(outputFile1.Name())
		}
	}

	// Test 2: Memory stats
	fmt.Printf("\n📊 Memory Statistics:\n")
	stats := media.MemoryStats()
	for key, value := range stats {
		fmt.Printf("   %s: %v\n", key, value)
	}

	// Test 3: Force cleanup and check again
	fmt.Printf("\n🧹 Memory Cleanup:\n")
	media.CleanupMemory()

	fmt.Printf("\n✨ Performance test completed!\n")
	fmt.Printf("⏰ Total time: %v\n", time.Since(start))
}

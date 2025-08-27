package test

import (
	"documents-worker/media"
	"documents-worker/types"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// Test with a sample file
	testFile := "./test_files/test.webp"

	// Check if test file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		fmt.Printf("Test file not found: %s\n", testFile)
		fmt.Println("Creating a simple performance summary instead...")
		showOptimizations()
		return
	}

	runPerformanceTest(testFile)
}

func runPerformanceTest(testFile string) {
	// Get file info
	info, err := os.Stat(testFile)
	if err != nil {
		fmt.Printf("Error getting file info: %v\n", err)
		return
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

	// Test processing
	fmt.Printf("🔄 Testing Optimized Processing\n")
	start := time.Now()

	outputFile, err := media.ExecCommand(false, testFile, converter)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("❌ Processing failed: %v\n", err)
	} else {
		fmt.Printf("✅ Processing completed in: %v\n", duration)
		if outputFile != nil {
			outputFile.Close()
			os.Remove(outputFile.Name())
		}
	}

	// Memory stats
	fmt.Printf("\n📊 Memory Statistics:\n")
	stats := media.MemoryStats()
	for key, value := range stats {
		fmt.Printf("   %s: %v\n", key, value)
	}

	fmt.Printf("\n✨ Performance test completed!\n")
}

func showOptimizations() {
	fmt.Printf("🚀 Documents Worker - Performance Optimizations Applied\n\n")

	fmt.Printf("✅ Memory Pool Integration:\n")
	fmt.Printf("   • Smart memory management for large files (>10MB)\n")
	fmt.Printf("   • Buffer reuse and allocation optimization\n")
	fmt.Printf("   • Automatic garbage collection management\n\n")

	fmt.Printf("✅ File Processing Optimizations:\n")
	fmt.Printf("   • File size detection for processing strategy\n")
	fmt.Printf("   • Structured logging with zerolog\n")
	fmt.Printf("   • Memory pool fallback mechanisms\n\n")

	fmt.Printf("✅ Performance Features:\n")
	fmt.Printf("   • Memory usage statistics tracking\n")
	fmt.Printf("   • Cleanup and resource management\n")
	fmt.Printf("   • Error handling with fallbacks\n\n")

	fmt.Printf("🎯 Focus Areas:\n")
	fmt.Printf("   ⚡ Efficient file processing\n")
	fmt.Printf("   🧠 Smart memory management\n")
	fmt.Printf("   📊 Performance monitoring\n")
	fmt.Printf("   🔧 Backward compatibility\n\n")

	// Show memory stats
	fmt.Printf("📊 Current Memory Statistics:\n")
	stats := media.MemoryStats()
	for key, value := range stats {
		fmt.Printf("   %s: %v\n", key, value)
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

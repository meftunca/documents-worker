package main

import (
	"fmt"
	"log"
	"time"

	"documents-worker/cmd/client"
)

func main() {
	// Create client with default configuration
	config := client.Config{
		BaseURL: "http://localhost:8080",
		Timeout: 30 * time.Second,
	}

	c := client.NewClient(config)

	// Check service health
	fmt.Println("🏥 Checking service health...")
	resp, err := c.Health()
	if err != nil {
		log.Printf("Health check failed: %v", err)
		fmt.Println("📝 Make sure the Documents Worker service is running on http://localhost:8080")
		return
	}

	if resp.Success {
		fmt.Println("✅ Service is healthy and ready!")
	} else {
		fmt.Printf("❌ Service health check failed: %s\n", resp.Message)
		return
	}

	// Example processing options
	imageOptions := &client.ProcessingOptions{
		Format:  "webp",
		Quality: 85,
		Width:   800,
		Height:  600,
		Metadata: map[string]string{
			"source": "example-client",
		},
	}

	documentOptions := &client.ProcessingOptions{
		Format: "pdf",
		Metadata: map[string]string{
			"client": "go-example",
		},
	}

	ocrOptions := &client.ProcessingOptions{
		Language: "tur",
		Page:     1,
	}

	// Examples (uncomment to test with actual files)
	fmt.Println("\n📚 Client Library Usage Examples:")
	fmt.Println("==================================")

	// Image processing example
	fmt.Println("\n🖼️  Image Processing:")
	fmt.Printf("client.ProcessImage(\"photo.jpg\", %+v)\n", imageOptions)

	// Document processing example
	fmt.Println("\n📄 Document Processing:")
	fmt.Printf("client.ProcessDocument(\"document.pdf\", %+v)\n", documentOptions)

	// OCR processing example
	fmt.Println("\n👁️  OCR Processing:")
	fmt.Printf("client.PerformOCR(\"scan.png\", %+v)\n", ocrOptions)

	// Text extraction example
	fmt.Println("\n📝 Text Extraction:")
	fmt.Println("client.ExtractText(\"document.docx\", nil)")

	// Job management examples
	fmt.Println("\n⚙️  Job Management:")
	fmt.Println("status, err := client.WaitForJob(jobID, 2*time.Second)")
	fmt.Println("result, err := client.GetJobResult(jobID)")

	// Batch processing example
	fmt.Println("\n📦 Batch Processing:")
	fmt.Println("files := []string{\"doc1.pdf\", \"doc2.docx\"}")
	fmt.Println("statuses, err := client.BatchProcess(files, \"document\", options, 5)")

	fmt.Println("\n🎯 To use these examples:")
	fmt.Println("1. Start the Documents Worker service")
	fmt.Println("2. Replace file paths with actual files")
	fmt.Println("3. Uncomment and run the desired operations")

	fmt.Println("\n📖 For more examples, see:")
	fmt.Println("- cmd/client/README.md")
	fmt.Println("- cmd/client-cli/README.md")
}

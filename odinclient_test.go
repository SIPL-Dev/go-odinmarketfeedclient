package ODINMarketFeed

import (
	"bytes"
	"testing"
)

// TestZLIBCompressor tests compression and decompression
func TestZLIBCompressor(t *testing.T) {
	compressor := &ZLIBCompressor{}
	testData := []byte("This is test data for compression")

	// Test compression
	compressed, err := compressor.Compress(testData)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}

	if len(compressed) == 0 {
		t.Fatal("Compressed data is empty")
	}

	// Test decompression
	decompressed, err := compressor.Uncompress(compressed)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}

	// Verify data integrity
	if !bytes.Equal(testData, decompressed) {
		t.Errorf("Decompressed data doesn't match original.\nExpected: %s\nGot: %s", testData, decompressed)
	}
}

// TestFragmentationHandler tests fragmentation handling
func TestFragmentationHandler(t *testing.T) {
	handler := NewFragmentationHandler()

	if handler == nil {
		t.Fatal("NewFragmentationHandler returned nil")
	}

	if handler.memoryStream == nil {
		t.Error("memoryStream not initialized")
	}

	if handler.zlibCompressor == nil {
		t.Error("zlibCompressor not initialized")
	}

	if handler.lastWrittenIndex != -1 {
		t.Errorf("Expected lastWrittenIndex to be -1, got %d", handler.lastWrittenIndex)
	}

	if handler.isDisposed {
		t.Error("Handler should not be disposed on creation")
	}
}

// TestFragmentData tests data fragmentation
func TestFragmentData(t *testing.T) {
	handler := NewFragmentationHandler()
	testData := []byte("Test message")

	fragmented, err := handler.FragmentData(testData)
	if err != nil {
		t.Fatalf("FragmentData failed: %v", err)
	}

	if len(fragmented) == 0 {
		t.Fatal("Fragmented data is empty")
	}

	// First byte should be compression flag (5)
	if fragmented[0] != 5 {
		t.Errorf("Expected compression flag 5, got %d", fragmented[0])
	}

	// Next 5 bytes should be length in ASCII
	lengthStr := string(fragmented[1:6])
	for i, ch := range lengthStr {
		if ch < '0' || ch > '9' {
			t.Errorf("Length character at position %d is not a digit: %c", i, ch)
		}
	}
}

// TestNewTradingWebSocket tests client creation
func TestNewTradingWebSocket(t *testing.T) {
	client := NewODINMarketFeedClient()

	if client == nil {
		t.Fatal("NewTradingWebSocket returned nil")
	}

	if client.fragHandler == nil {
		t.Error("fragHandler not initialized")
	}

	if client.isDisposed {
		t.Error("Client should not be disposed on creation")
	}

	// if client.compression != CompressionON {
	// 	t.Errorf("Expected compression to be ON, got %v", client.compression)
	// }
}

// TestIsConnected tests connection status
func TestIsConnected(t *testing.T) {
	//client := NewODINMarketFeedClient()

	// Should not be connected initially
	// if client.IsConnected() {
	// 	t.Error("Client should not be connected on creation")
	// }
}

// BenchmarkZLIBCompress benchmarks compression performance
func BenchmarkZLIBCompress(b *testing.B) {
	compressor := &ZLIBCompressor{}
	testData := []byte("This is test data for compression benchmarking with some repeated text. This is test data for compression benchmarking with some repeated text.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compressor.Compress(testData)
	}
}

// BenchmarkZLIBUncompress benchmarks decompression performance
func BenchmarkZLIBUncompress(b *testing.B) {
	compressor := &ZLIBCompressor{}
	testData := []byte("This is test data for compression benchmarking with some repeated text. This is test data for compression benchmarking with some repeated text.")
	compressed, _ := compressor.Compress(testData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compressor.Uncompress(compressed)
	}
}

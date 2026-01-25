package mobi

import (
	"os"
	"testing"
)

func TestConvertSpecificBook(t *testing.T) {
	// 这是一个集成测试，依赖于本地特定文件
	mobiPath := `../../../很纯很暧昧.mobi`
	epubPath := `../../../很纯很暧昧.epub`

	// 1. 检查源文件是否存在
	if _, err := os.Stat(mobiPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: source file not found at %s", mobiPath)
	}

	// 2. 初始化 Reader
	reader, err := NewReader(mobiPath)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	// 3. 验证基本元数据
	title := string(reader.MobiBook.Headers.MobiHeader.Title)
	t.Logf("Book Title: %s", title)
	if title == "" {
		t.Error("Book title should not be empty")
	}

	// 4. 测试文本提取 (抽样)
	text, err := reader.ExtractText()
	if err != nil {
		t.Fatalf("Failed to extract text: %v", err)
	}
	if len(text) < 100 {
		t.Errorf("Extracted text is too short (len=%d), expected > 100", len(text))
	}
	t.Logf("Extracted text length: %d", len(text))
	t.Logf("Text sample: %s...", text[:min(100, len(text))])

	// 5. 测试转换为 EPUB
	err = reader.ToEpub(epubPath)
	if err != nil {
		t.Fatalf("Failed to convert to EPUB: %v", err)
	}

	// 6. 验证输出文件
	info, err := os.Stat(epubPath)
	if err != nil {
		t.Fatalf("Output EPUB file was not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Output EPUB file is empty")
	}
	t.Logf("Successfully converted to %s (Size: %d bytes)", epubPath, info.Size())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

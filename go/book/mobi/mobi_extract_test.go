package mobi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractMethods(t *testing.T) {
	// 使用本地存在的特定测试文件
	mobiPath := `../../../test/诡舍.azw3`

	// 1. 检查文件是否存在
	if _, err := os.Stat(mobiPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: source file not found at %s", mobiPath)
	}

	// 2. 初始化 Reader
	reader, err := NewReader(mobiPath)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	t.Run("TestExtractHtml", func(t *testing.T) {
		htmlContent, err := reader.ExtractHtml()
		if err != nil {
			t.Fatalf("ExtractHtml failed: %v", err)
		}

		if len(htmlContent) == 0 {
			t.Error("Extracted HTML is empty")
		}

		// 验证是否包含常见的 HTML 标签
		if !strings.Contains(htmlContent, "<") || !strings.Contains(htmlContent, ">") {
			t.Error("Extracted content does not look like HTML (missing brackets)")
		}

		// 打印前 200 个字符用于人工确认
		//sampleLen := min(200, len(htmlContent))
		//t.Logf("HTML Sample (first %d chars):\n%s", sampleLen, htmlContent[:sampleLen])

		// 生成同名 html 文件
		htmlPath := strings.Replace(mobiPath, filepath.Ext(mobiPath), ".html", 1)
		err = os.WriteFile(htmlPath, []byte(htmlContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write HTML file: %v", err)
		}
		t.Logf("Generated HTML file at: %s", htmlPath)
	})

	t.Run("TestExtractText", func(t *testing.T) {
		textContent, err := reader.ExtractText()
		if err != nil {
			t.Fatalf("ExtractText failed: %v", err)
		}

		if len(textContent) == 0 {
			t.Error("Extracted Text is empty")
		}

		// 验证是否移除了部分 HTML 标签 (例如 <p> 或 <br>)
		// 注意：文本中可能包含 '<' 字符本身，所以不能单纯判断是否有 '<'
		// 但通常不应包含完整的 HTML 标签结构
		if strings.Contains(textContent, "<html>") || strings.Contains(textContent, "<body>") {
			t.Error("Extracted text still contains structure tags like <html> or <body>")
		}

		// 打印前 200 个字符用于人工确认
		sampleLen := min(200, len(textContent))
		t.Logf("Text Sample (first %d chars):\n%s", sampleLen, textContent[:sampleLen])

		// 生成同名 txt 文件
		txtPath := strings.Replace(mobiPath, filepath.Ext(mobiPath), ".txt", 1)
		err = os.WriteFile(txtPath, []byte(textContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write TXT file: %v", err)
		}
		t.Logf("Generated TXT file at: %s", txtPath)
	})
}

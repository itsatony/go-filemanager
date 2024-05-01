package filemanager

import (
	"bytes"
	"fmt"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

type PDFTextExtractorPlugin struct{}

func (p *PDFTextExtractorPlugin) Process(files []*ManagedFile) ([]*ManagedFile, error) {
	var processedFiles []*ManagedFile

	for _, file := range files {
		if !isPDFFile(file) {
			processedFiles = append(processedFiles, file)
			continue
		}

		reader := bytes.NewReader(file.Content)
		pdfReader, err := model.NewPdfReader(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read PDF: %v", err)
		}

		numPages, err := pdfReader.GetNumPages()
		if err != nil {
			return nil, fmt.Errorf("failed to get number of pages: %v", err)
		}

		var extractedText []string

		for i := 0; i < numPages; i++ {
			page, err := pdfReader.GetPage(i + 1)
			if err != nil {
				return nil, fmt.Errorf("failed to get page %d: %v", i+1, err)
			}

			ex, err := extractor.New(page)
			if err != nil {
				return nil, fmt.Errorf("failed to create extractor: %v", err)
			}

			text, err := ex.ExtractText()
			if err != nil {
				return nil, fmt.Errorf("failed to extract text: %v", err)
			}

			extractedText = append(extractedText, text)
		}

		outputFormat := file.MetaData["output_format"].(string)

		var outputContent []byte
		switch outputFormat {
		case "text":
			outputContent = []byte(strings.Join(extractedText, "\n"))
		case "markdown":
			html := convertToHTML(extractedText)
			converter := md.NewConverter("", true, nil)
			markdown, err := converter.ConvertString(html)
			if err != nil {
				return nil, fmt.Errorf("failed to convert HTML to Markdown: %v", err)
			}
			outputContent = []byte(markdown)
		default:
			return nil, fmt.Errorf("unsupported output format: %s", outputFormat)
		}

		file.Content = outputContent
		file.MimeType = "text/plain"
		file.FileName = fmt.Sprintf("%s.%s", strings.TrimSuffix(file.FileName, ".pdf"), outputFormat)

		processedFiles = append(processedFiles, file)
	}

	return processedFiles, nil
}

func isPDFFile(file *ManagedFile) bool {
	return file.MimeType == "application/pdf"
}

func convertToHTML(lines []string) string {
	var htmlLines []string

	htmlLines = append(htmlLines, "<html><body>")
	for _, line := range lines {
		htmlLines = append(htmlLines, "<p>"+line+"</p>")
	}
	htmlLines = append(htmlLines, "</body></html>")

	return strings.Join(htmlLines, "\n")
}

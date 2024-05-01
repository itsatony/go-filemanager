package filemanager

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type FormatConverterPlugin struct{}

func (p *FormatConverterPlugin) Process(files []*ManagedFile) ([]*ManagedFile, error) {
	var processedFiles []*ManagedFile

	for _, file := range files {
		var convertedContent []byte
		var err error

		switch strings.ToLower(file.MimeType) {
		case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
			convertedContent, err = convertDocxToText(file.Content)
			if err != nil {
				convertedContent, err = convertDocxToMarkdown(file.Content)
			}
		case "application/vnd.ms-excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
			convertedContent, err = convertExcelToCSV(file.Content)
		default:
			processedFiles = append(processedFiles, file)
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("failed to convert file format: %v", err)
		}

		convertedFile := &ManagedFile{
			FileName:         file.FileName,
			Content:          convertedContent,
			MimeType:         "text/plain",
			FileSize:         int64(len(convertedContent)),
			MetaData:         file.MetaData,
			ProcessingErrors: []string{},
		}

		processedFiles = append(processedFiles, convertedFile)
	}

	return processedFiles, nil
}

func convertDocxToText(content []byte) ([]byte, error) {
	// Convert DOCX to plain text using a library or custom implementation
	// Here's a placeholder implementation that assumes the content is already in plain text format
	return content, nil
}

func convertDocxToMarkdown(content []byte) ([]byte, error) {
	// Convert DOCX to Markdown using the goldmark library
	var buf bytes.Buffer
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)
	if err := md.Convert(content, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func convertExcelToCSV(content []byte) ([]byte, error) {
	// Load the Excel file
	xlsx, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	// Get the first sheet name
	sheetName := xlsx.GetSheetName(1)

	// Get all the rows in the sheet
	rows, err := xlsx.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	// Create a new CSV writer
	var csvBuf bytes.Buffer
	csvWriter := csv.NewWriter(&csvBuf)

	// Write the rows to the CSV writer
	for _, row := range rows {
		if err := csvWriter.Write(row); err != nil {
			return nil, err
		}
	}

	csvWriter.Flush()

	if err := csvWriter.Error(); err != nil {
		return nil, err
	}

	return csvBuf.Bytes(), nil
}

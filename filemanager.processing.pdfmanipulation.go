package filemanager

import (
	"bytes"
	"fmt"
	"time"

	"github.com/unidoc/unipdf/v3/model"
	"github.com/unidoc/unipdf/v3/model/optimize"
)

type PDFManipulationPlugin struct{}

func (p *PDFManipulationPlugin) Process(files []*ManagedFile, fileProcess *FileProcess) ([]*ManagedFile, error) {
	var processedFiles []*ManagedFile

	for _, file := range files {
		if !isPDFFile(file) {
			processedFiles = append(processedFiles, file)
			continue
		}
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "PDFManipulation",
			StatusDescription: fmt.Sprintf("Manipulating PDF: %s", file.FileName),
		}
		fileProcess.AddProcessingUpdate(status)
		reader := bytes.NewReader(file.Content)
		pdfReader, err := model.NewPdfReader(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read PDF: %v", err)
		}

		manipulationType := file.MetaData["manipulation_type"].(string)

		switch manipulationType {
		case "extract":
			extractedFile, err := extractPages(pdfReader, file.MetaData)
			if err != nil {
				return nil, err
			}
			processedFiles = append(processedFiles, extractedFile)
		case "merge":
			mergedFile, err := mergePDFs(pdfReader, files, file.MetaData)
			if err != nil {
				return nil, err
			}
			processedFiles = append(processedFiles, mergedFile)
		case "compress":
			compressedFile, err := compressPDF(pdfReader, file.MetaData)
			if err != nil {
				return nil, err
			}
			processedFiles = append(processedFiles, compressedFile)
		case "reorder":
			reorderedFile, err := reorderPages(pdfReader, file.MetaData)
			if err != nil {
				return nil, err
			}
			processedFiles = append(processedFiles, reorderedFile)
		default:
			return nil, fmt.Errorf("unsupported manipulation type: %s", manipulationType)
		}
	}

	return processedFiles, nil
}

// func isPDFFile(file *ManagedFile) bool {
// 	return file.MimeType == "application/pdf"
// }

func extractPages(pdfReader *model.PdfReader, metaData map[string]interface{}) (*ManagedFile, error) {
	startPage := int(metaData["start_page"].(float64))
	endPage := int(metaData["end_page"].(float64))

	numberOfPages, err := pdfReader.GetNumPages()
	if err != nil {
		return nil, fmt.Errorf("failed to get number of pages: %v", err)
	}
	if startPage < 1 || endPage > numberOfPages || startPage > endPage {
		return nil, fmt.Errorf("invalid page range: start=%d, end=%d", startPage, endPage)
	}

	pdfWriter := model.NewPdfWriter()

	for i := startPage; i <= endPage; i++ {
		page, err := pdfReader.GetPage(i)
		if err != nil {
			return nil, fmt.Errorf("failed to get page %d: %v", i, err)
		}

		err = pdfWriter.AddPage(page)
		if err != nil {
			return nil, fmt.Errorf("failed to add page %d to writer: %v", i, err)
		}
	}

	var buf bytes.Buffer
	err = pdfWriter.Write(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to write PDF: %v", err)
	}

	extractedFile := &ManagedFile{
		FileName:         fmt.Sprintf("extracted_%d-%d.pdf", startPage, endPage),
		Content:          buf.Bytes(),
		MimeType:         "application/pdf",
		FileSize:         int64(buf.Len()),
		MetaData:         metaData,
		ProcessingErrors: []string{},
	}

	return extractedFile, nil
}

func mergePDFs(pdfReader *model.PdfReader, files []*ManagedFile, metaData map[string]interface{}) (*ManagedFile, error) {
	mergeFileNames := metaData["merge_files"].([]interface{})

	pdfWriter := model.NewPdfWriter()

	// Add pages from the base PDF
	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return nil, fmt.Errorf("failed to get number of pages: %v", err)
	}

	for i := 1; i <= numPages; i++ {
		page, err := pdfReader.GetPage(i)
		if err != nil {
			return nil, fmt.Errorf("failed to get page %d: %v", i, err)
		}

		err = pdfWriter.AddPage(page)
		if err != nil {
			return nil, fmt.Errorf("failed to add page %d to writer: %v", i, err)
		}
	}

	// Merge pages from the specified files
	for _, fileName := range mergeFileNames {
		mergeFile := findFileByName(files, fileName.(string))
		if mergeFile == nil {
			return nil, fmt.Errorf("merge file not found: %s", fileName)
		}

		mergeReader, err := model.NewPdfReader(bytes.NewReader(mergeFile.Content))
		if err != nil {
			return nil, fmt.Errorf("failed to read merge file: %v", err)
		}

		numPages, err := mergeReader.GetNumPages()
		if err != nil {
			return nil, fmt.Errorf("failed to get number of pages in merge file: %v", err)
		}

		for i := 1; i <= numPages; i++ {
			page, err := mergeReader.GetPage(i)
			if err != nil {
				return nil, fmt.Errorf("failed to get page %d from merge file: %v", i, err)
			}

			err = pdfWriter.AddPage(page)
			if err != nil {
				return nil, fmt.Errorf("failed to add page %d from merge file to writer: %v", i, err)
			}
		}
	}

	var buf bytes.Buffer
	err = pdfWriter.Write(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to write merged PDF: %v", err)
	}

	mergedFile := &ManagedFile{
		FileName:         "merged.pdf",
		Content:          buf.Bytes(),
		MimeType:         "application/pdf",
		FileSize:         int64(buf.Len()),
		MetaData:         metaData,
		ProcessingErrors: []string{},
	}

	return mergedFile, nil
}

func findFileByName(files []*ManagedFile, fileName string) *ManagedFile {
	for _, file := range files {
		if file.FileName == fileName {
			return file
		}
	}
	return nil
}

func compressPDF(pdfReader *model.PdfReader, metaData map[string]interface{}) (*ManagedFile, error) {
	compressionLevel := metaData["compression_level"].(string)

	// Create a new PDF writer
	pdfWriter := model.NewPdfWriter()

	// Set the compression level based on the provided metadata
	switch compressionLevel {
	case "low":
		pdfWriter.SetOptimizer(optimize.New(optimize.Options{
			CombineDuplicateDirectObjects:   true,
			CombineIdenticalIndirectObjects: true,
			CombineDuplicateStreams:         true,
			CompressStreams:                 true,
			UseObjectStreams:                true,
			ImageQuality:                    90,
			ImageUpperPPI:                   150,
		}))
	case "medium":
		pdfWriter.SetOptimizer(optimize.New(optimize.Options{
			CombineDuplicateDirectObjects:   true,
			CombineIdenticalIndirectObjects: true,
			CombineDuplicateStreams:         true,
			CompressStreams:                 true,
			UseObjectStreams:                true,
			ImageQuality:                    80,
			ImageUpperPPI:                   100,
		}))
	case "high":
		pdfWriter.SetOptimizer(optimize.New(optimize.Options{
			CombineDuplicateDirectObjects:   true,
			CombineIdenticalIndirectObjects: true,
			CombineDuplicateStreams:         true,
			CompressStreams:                 true,
			UseObjectStreams:                true,
			ImageQuality:                    70,
			ImageUpperPPI:                   50,
		}))
	default:
		return nil, fmt.Errorf("invalid compression level: %s", compressionLevel)
	}

	// Add pages from the original PDF to the writer
	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return nil, fmt.Errorf("failed to get number of pages: %v", err)
	}

	for i := 1; i <= numPages; i++ {
		page, err := pdfReader.GetPage(i)
		if err != nil {
			return nil, fmt.Errorf("failed to get page %d: %v", i, err)
		}

		err = pdfWriter.AddPage(page)
		if err != nil {
			return nil, fmt.Errorf("failed to add page %d to writer: %v", i, err)
		}
	}

	// Write the compressed PDF to a buffer
	var buf bytes.Buffer
	err = pdfWriter.Write(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to write compressed PDF: %v", err)
	}

	compressedFile := &ManagedFile{
		FileName:         "compressed.pdf",
		Content:          buf.Bytes(),
		MimeType:         "application/pdf",
		FileSize:         int64(buf.Len()),
		MetaData:         metaData,
		ProcessingErrors: []string{},
	}

	return compressedFile, nil
}

func reorderPages(pdfReader *model.PdfReader, metaData map[string]interface{}) (*ManagedFile, error) {
	pageOrder := metaData["page_order"].([]interface{})

	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return nil, fmt.Errorf("failed to get number of pages: %v", err)
	}

	// Create a map to store the original page number and its corresponding page object
	pageMap := make(map[int]model.PdfPage)
	for i := 1; i <= numPages; i++ {
		page, err := pdfReader.GetPage(i)
		if err != nil {
			return nil, fmt.Errorf("failed to get page %d: %v", i, err)
		}
		pageMap[i] = *page
	}

	// Create a new PDF writer
	pdfWriter := model.NewPdfWriter()

	// Add pages to the writer in the specified order
	for _, pageNum := range pageOrder {
		pageNumber := int(pageNum.(float64))
		page, ok := pageMap[pageNumber]
		if !ok {
			return nil, fmt.Errorf("invalid page number: %d", pageNumber)
		}

		err = pdfWriter.AddPage(&page)
		if err != nil {
			return nil, fmt.Errorf("failed to add page %d to writer: %v", pageNumber, err)
		}
	}

	// Write the reordered PDF to a buffer
	var buf bytes.Buffer
	err = pdfWriter.Write(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to write reordered PDF: %v", err)
	}

	reorderedFile := &ManagedFile{
		FileName:         "reordered.pdf",
		Content:          buf.Bytes(),
		MimeType:         "application/pdf",
		FileSize:         int64(buf.Len()),
		MetaData:         metaData,
		ProcessingErrors: []string{},
	}

	return reorderedFile, nil
}

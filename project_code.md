# project_code.md

## ./filemanager.helpers.go

```go
package filemanager

import (
	"strconv"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

const idAlphabet string = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_"

func NID(prefix string, length int) (nid string) {
	nid, err := gonanoid.Generate(idAlphabet, length)
	if err != nil {
		nid = strconv.FormatInt(time.Now().UnixMicro(), 10)
	}
	if len(prefix) > 0 {
		nid = prefix + "_" + nid
	}
	return nid
}
```

## ./filemanager.processor.imagemanipulation.go

```go
package filemanager

import (
	"bytes"
	"fmt"
	"image"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
)

type ImageManipulationPlugin struct{}

func (p *ImageManipulationPlugin) Process(files []*ManagedFile, fileProcess *FileProcess) ([]*ManagedFile, error) {
	var processedFiles []*ManagedFile

	for _, file := range files {
		if !isImageFile(file) {
			processedFiles = append(processedFiles, file)
			continue
		}
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "ImageManipulation",
			StatusDescription: fmt.Sprintf("Processing file(%s)", file.FileName),
			Error:             nil,
		}
		fileProcess.AddProcessingUpdate(status)
		img, err := imaging.Decode(bytes.NewReader(file.Content))
		if err != nil {
			return nil, fmt.Errorf("failed to decode image: %v", err)
		}

		// Perform image manipulation based on the specified parameters
		params := file.MetaData
		if val, ok := params["format"]; ok {
			format, ok := val.(string)
			if !ok {
				return nil, fmt.Errorf("invalid format parameter: %v", val)
			}
			img, err = convertImageFormat(img, format)
			if err != nil {
				return nil, err
			}
			file.MimeType = mime.TypeByExtension("." + format)
			file.FileName = fmt.Sprintf("%s.%s", strings.TrimSuffix(file.FileName, filepath.Ext(file.FileName)), format)
		}

		if val, ok := params["width"]; ok {
			widthFloat, ok := val.(float64)
			if !ok {
				return nil, fmt.Errorf("invalid width parameter: %v", val)
			}
			width := int(widthFloat)
			img = imaging.Resize(img, width, 0, imaging.Lanczos)
		}

		if val, ok := params["height"]; ok {
			heightFloat, ok := val.(float64)
			if !ok {
				return nil, fmt.Errorf("invalid height parameter: %v", val)
			}
			height := int(heightFloat)
			img = imaging.Resize(img, 0, height, imaging.Lanczos)
		}

		if val, ok := params["aspect_ratio"]; ok {
			aspectRatio, ok := val.(string)
			if !ok {
				return nil, fmt.Errorf("invalid aspect_ratio parameter: %v", val)
			}
			img, err = cropToAspectRatio(img, aspectRatio)
			if err != nil {
				return nil, err
			}
		}

		// Encode the processed image
		var buf bytes.Buffer
		format, err := imaging.FormatFromExtension(filepath.Ext(file.FileName))
		if err != nil {
			return nil, fmt.Errorf("unsupported image format: %v", err)
		}
		err = imaging.Encode(&buf, img, format)
		if err != nil {
			return nil, fmt.Errorf("failed to encode image: %v", err)
		}

		file.Content = buf.Bytes()
		processedFiles = append(processedFiles, file)
	}

	return processedFiles, nil
}

func isImageFile(file *ManagedFile) bool {
	mimeType := file.MimeType
	return strings.HasPrefix(mimeType, "image/")
}

func convertImageFormat(img image.Image, format string) (image.Image, error) {
	switch format {
	case "jpg", "jpeg":
		return img, nil
	case "png":
		return img, nil
	case "webp":
		return img, nil
	default:
		return nil, fmt.Errorf("unsupported image format: %s", format)
	}
}

func cropToAspectRatio(img image.Image, aspectRatio string) (image.Image, error) {
	width, height := getAspectRatioDimensions(img, aspectRatio)
	return imaging.Fill(img, width, height, imaging.Center, imaging.Lanczos), nil
}

func getAspectRatioDimensions(img image.Image, aspectRatio string) (int, int) {
	bounds := img.Bounds()
	imgWidth, imgHeight := bounds.Dx(), bounds.Dy()

	switch aspectRatio {
	case "1:1":
		size := min(imgWidth, imgHeight)
		return size, size
	case "4:3":
		return 4 * imgHeight / 3, imgHeight
	case "16:9":
		return 16 * imgHeight / 9, imgHeight
	case "21:9":
		return 21 * imgHeight / 9, imgHeight
	default:
		return imgWidth, imgHeight
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

## ./filemanager.processing.formatconverter.go

```go
package filemanager

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type FormatConverterPlugin struct{}

func (p *FormatConverterPlugin) Process(files []*ManagedFile, fileProcess *FileProcess) ([]*ManagedFile, error) {
	var processedFiles []*ManagedFile

	for _, file := range files {
		var convertedContent []byte
		var err error

		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "FormatConverter",
			StatusDescription: fmt.Sprintf("Converting file format: %s", file.FileName),
		}
		fileProcess.AddProcessingUpdate(status)

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
```

## ./filemanager.go

```go
// file_manager.go

// Package filemanager provides a flexible and extensible file management system
// for handling file storage, retrieval, and processing using a plugin-based architecture.
//
// The main components of the filemanager package are:
//
// - FileManager: The central structure that manages file storage, retrieval, and processing.
//   It provides methods for initializing the FileManager, adding processing plugins,
//   loading recipes, and processing files.
//
// - ManagedFile: Represents a file managed by the FileManager. It contains information
//   such as the file name, MIME type, URL, local file path, file size, metadata, and
//   processing errors.
//
// - FileProcess: Represents a file processing task. It includes the incoming file name,
//   recipe name, and processing updates. It provides methods for adding processing updates
//   and retrieving the latest processing status.
//
// - ProcessingPlugin: An interface that defines the contract for processing plugins.
//   Processing plugins are responsible for processing files based on specific requirements.
//
// - Recipe: Represents a processing recipe that specifies the accepted MIME types, file size
//   constraints, processing steps, and output formats for a file processing task.
//
// - ProcessingStatus: Represents the status of a file processing task. It includes information
//   such as the process ID, timestamp, processor name, status description, progress percentage,
//   error (if any), completion status, and resulting files.
//
// - ProcessingResultFile: Represents a resulting file from a file processing task. It contains
//   information such as the file name, local file path, URL, file size, and MIME type.
//
// The filemanager package provides a high-level API for managing files and processing them
// using various plugins and recipes. It abstracts the complexities of file storage and
// processing, allowing developers to focus on defining processing plugins and recipes to
// suit their specific requirements.

package filemanager

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gabriel-vasile/mimetype"
	"gopkg.in/yaml.v2"
)

var (
	ErrLocalFileNotFound = errors.New("local file not found")
	ErrUrlNotMapped      = errors.New("url not mapped to local file")
)

const FILE_PROCESS_ID_LENGTH = 16
const FILE_PROCESS_ID_PREFIX = "FP"

type FileStorageType string

const (
	FileStorageTypePrivate FileStorageType = "private"
	FileStorageTypeTemp    FileStorageType = "temp"
	FileStorageTypePublic  FileStorageType = "public"
)

type FileProcess struct {
	ID                string
	IncomingFileName  string
	RecipeName        string
	ProcessingUpdates []ProcessingStatus
	LatestStatus      *ProcessingStatus
}

func (fp *FileProcess) AddProcessingUpdate(update ProcessingStatus) {
	fp.ProcessingUpdates = append(fp.ProcessingUpdates, update)
	fp.LatestStatus = &update
}

func (fp *FileProcess) GetLatestProcessingStatus() *ProcessingStatus {
	return fp.LatestStatus
}

func NewFileProcess(incomingFileName, recipeName string) *FileProcess {
	id := NID(FILE_PROCESS_ID_PREFIX, FILE_PROCESS_ID_LENGTH)
	return &FileProcess{
		ID:               id,
		IncomingFileName: incomingFileName,
		RecipeName:       recipeName,
	}
}

type FileManager struct {
	publicLocalBasePath  string
	privateLocalBasePath string
	baseUrl              string
	localTempPath        string
	processingPlugins    map[string]ProcessingPlugin
	recipes              map[string]Recipe
	mu                   sync.RWMutex
}

func NewFileManager(publicLocalBasePath, privateLocalBasePath, baseUrl, tempPath string) *FileManager {
	return &FileManager{
		publicLocalBasePath:  publicLocalBasePath,
		privateLocalBasePath: privateLocalBasePath,
		baseUrl:              baseUrl,
		localTempPath:        tempPath,
		processingPlugins:    make(map[string]ProcessingPlugin),
		recipes:              make(map[string]Recipe),
	}
}

func (fm *FileManager) AddProcessingPlugin(name string, plugin ProcessingPlugin) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.processingPlugins[name] = plugin
}

func (fm *FileManager) LoadRecipes(recipesDir string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	files, err := os.ReadDir(recipesDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if filepath.Ext(file.Name()) != ".yaml" {
			continue
		}

		filePath := filepath.Join(recipesDir, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var recipe Recipe
		err = yaml.Unmarshal(data, &recipe)
		if err != nil {
			continue
		}

		fm.recipes[recipe.Name] = recipe
	}

	return nil
}

func (fm *FileManager) GetRecipe(name string) (Recipe, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	if _, ok := fm.recipes[name]; !ok {
		return Recipe{}, ErrRecipeNotFound
	}
	return fm.recipes[name], nil
}

func (aifm *FileManager) GetLocalPathForFile(target FileStorageType, filename string) string {
	var localPath string
	switch target {
	case FileStorageTypePrivate:
		localPath = aifm.GetPrivateLocalFilePath(filename)
	case FileStorageTypeTemp:
		localPath = aifm.GetLocalTemporaryFilePath(filename)
	case FileStorageTypePublic:
		localPath = aifm.GetPublicLocalFilePath(filename)
	}
	return localPath
}

func (aifm *FileManager) GetPublicUrlForFile(localFilePath string) (url string, err error) {
	// first check if the local file path has our local public base path - if not, return error
	if !strings.HasPrefix(localFilePath, aifm.publicLocalBasePath) {
		return url, ErrLocalFileNotFound
	}
	relativePath := strings.TrimPrefix(localFilePath, aifm.publicLocalBasePath)
	return path.Join(aifm.baseUrl, relativePath), nil
}

func (aifm *FileManager) GetPublicLocalBasePath() string {
	return aifm.publicLocalBasePath
}

func (aifm *FileManager) GetPrivateLocalBasePath() string {
	return aifm.privateLocalBasePath
}

func (aifm *FileManager) GetBaseUrl() string {
	return aifm.baseUrl
}

func (aifm *FileManager) GetLocalPathOfUrl(url string) (localPath string, err error) {
	// first check if the url has our url prefix - if not, return error
	if !strings.HasPrefix(url, aifm.baseUrl) {
		return localPath, ErrUrlNotMapped
	}
	// get the relative path and filename from the url and append it to the local base path
	relativePath := strings.TrimPrefix(url, aifm.baseUrl)
	localPath = path.Join(aifm.publicLocalBasePath, relativePath)
	// check if the file exists
	if !FileExists(localPath) {
		return localPath, ErrLocalFileNotFound
	}
	return localPath, nil
}

func (aifm *FileManager) GetPublicLocalFilePath(fileName string) string {
	return path.Join(aifm.publicLocalBasePath, fileName)
}

func (aifm *FileManager) GetPrivateLocalFilePath(fileName string) string {
	return path.Join(aifm.privateLocalBasePath, fileName)
}

func (aifm *FileManager) GetLocalTemporaryPath() string {
	return aifm.localTempPath
}

func (aifm *FileManager) GetLocalTemporaryFilePath(fileName string) string {
	return path.Join(aifm.localTempPath, fileName)
}

func GuessMimeType(filepath string) (string, error) {
	mtype, err := mimetype.DetectFile(filepath)
	if err != nil {
		return "", err
	}
	mime := mtype.String()
	return mime, err
}

func DownloadFileFromUrl(url string, localFilePath string) (err error) {
	// Download the file from url
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	file, err := os.Create(localFilePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}
	return nil
}

func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}
```

## ./filemanager.upload.go

```go
// upload.go
package filemanager

import (
	"fmt"
	"io"
	"os"
	"time"
)

func (fm *FileManager) HandleFileUpload(r io.Reader, fileProcess *FileProcess, statusCh chan<- *FileProcess) (*ManagedFile, error) {
	tempFile, err := os.CreateTemp(fm.localTempPath, "upload-*")
	if err != nil {
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "FileUpload",
			StatusDescription: "Failed to create temporary file",
			Error:             err,
			Done:              true,
		}
		fileProcess.AddProcessingUpdate(status)
		statusCh <- fileProcess
		return nil, err
	}
	defer tempFile.Close()

	progressReader := &ProgressReader{
		Reader:      r,
		Size:        0,
		Uploaded:    0,
		StatusCh:    statusCh,
		FileProcess: fileProcess,
	}

	_, err = io.Copy(tempFile, progressReader)
	if err != nil {
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "FileUpload",
			StatusDescription: "Failed to save uploaded file",
			Error:             err,
			Done:              true,
		}
		fileProcess.AddProcessingUpdate(status)
		statusCh <- fileProcess
		return nil, err
	}

	managedFile := &ManagedFile{
		FileName:      tempFile.Name(),
		LocalFilePath: tempFile.Name(),
	}

	managedFile.UpdateMimeType()
	managedFile.UpdateFilesize()

	resultingFile := ProcessingResultFile{
		FileName:      managedFile.FileName,
		LocalFilePath: managedFile.LocalFilePath,
		FileSize:      managedFile.FileSize,
		MimeType:      managedFile.MimeType,
	}

	status := ProcessingStatus{
		ProcessID:         fileProcess.ID,
		TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
		ProcessorName:     "FileUpload",
		StatusDescription: "File uploaded successfully",
		Done:              false,
		ResultingFiles:    []ProcessingResultFile{resultingFile},
	}
	fileProcess.AddProcessingUpdate(status)
	statusCh <- fileProcess

	return managedFile, nil
}

type ProgressReader struct {
	Reader      io.Reader
	Size        int64
	Uploaded    int64
	StatusCh    chan<- *FileProcess
	FileProcess *FileProcess
}

func (r *ProgressReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.Uploaded += int64(n)

	if r.Size == 0 {
		if file, ok := r.Reader.(*os.File); ok {
			fileInfo, err := file.Stat()
			if err == nil {
				r.Size = fileInfo.Size()
			}
		}
	}

	if r.Size > 0 {
		percentage := int(float64(r.Uploaded) / float64(r.Size) * 100)
		status := ProcessingStatus{
			ProcessID:         r.FileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "FileUpload",
			StatusDescription: fmt.Sprintf("Uploading file: %s", r.FileProcess.IncomingFileName),
			Percentage:        percentage,
		}
		r.FileProcess.AddProcessingUpdate(status)
		select {
		case r.StatusCh <- r.FileProcess:
		default:
		}
	}

	return n, err
}
```

## ./filemanager.processing.pdfmanipulation.go

```go
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
```

## ./filemanager.processor.clamav.go

```go
package filemanager

import (
	"bytes"
	"fmt"
	"time"

	"github.com/dutchcoders/go-clamd"
)

type ClamAVPlugin struct {
	clam *clamd.Clamd
}

// NewClamAVPlugin creates a new ClamAVPlugin instance - only works with TCP connection
// tcp := viper.GetString("CLAMAV_TCP")
func NewClamAVPlugin(tcpConnection string) (*ClamAVPlugin, error) {
	var clam *clamd.Clamd
	var err error

	clam = clamd.NewClamd(tcpConnection)

	err = clam.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClamAV: %v", err)
	}

	return &ClamAVPlugin{clam: clam}, nil
}

func (p *ClamAVPlugin) Process(files []*ManagedFile, fileProcess *FileProcess) ([]*ManagedFile, error) {
	var processedFiles []*ManagedFile

	for _, file := range files {
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "ClamAV",
			StatusDescription: fmt.Sprintf("Scanning file for viruses: %s", file.FileName),
		}
		fileProcess.AddProcessingUpdate(status)
		scanResultChan, err := p.clam.ScanStream(bytes.NewReader(file.Content), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file: %v", err)
		}

		scanResult := <-scanResultChan

		if scanResult.Status != "OK" {
			file.ProcessingErrors = append(file.ProcessingErrors, fmt.Sprintf("virus detected: %s", scanResult.Description))
		}

		processedFiles = append(processedFiles, file)
	}

	return processedFiles, nil
}
```

## ./filemanager.processing.exifmetadata.go

```go
package filemanager

import (
	"bytes"
	"fmt"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

type ExifMetadataExtractorPlugin struct{}

func (p *ExifMetadataExtractorPlugin) Process(files []*ManagedFile, fileProcess *FileProcess) ([]*ManagedFile, error) {
	var processedFiles []*ManagedFile

	for _, file := range files {
		if !isImageFile(file) {
			processedFiles = append(processedFiles, file)
			continue
		}
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "ExifMetadataExtractor",
			StatusDescription: fmt.Sprintf("Extracting Exif metadata from image: %s", file.FileName),
		}
		fileProcess.AddProcessingUpdate(status)
		exifData, err := extractExifMetadata(file.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to extract Exif metadata: %v", err)
		}

		file.MetaData["exif"] = exifData
		processedFiles = append(processedFiles, file)
	}

	return processedFiles, nil
}

// func isImageFile(file *ManagedFile) bool {
// 	mimeType := file.MimeType
// 	return strings.HasPrefix(mimeType, "image/")
// }

func extractExifMetadata(content []byte) (map[string]string, error) {
	exifData := make(map[string]string)

	x, err := exif.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	fields := []exif.FieldName{
		exif.Make,
		exif.Model,
		exif.DateTime,
		exif.GPSLatitude,
		exif.GPSLongitude,
		exif.GPSAltitude,
		exif.FocalLength,
		exif.FNumber,
		exif.ExposureTime,
		exif.ISOSpeedRatings,
	}

	for _, field := range fields {
		tag, err := x.Get(field)
		if err == nil {
			exifData[string(field)] = tag.String()
		}
	}

	return exifData, nil
}
```

## ./filemanager.processing.pdftextextract.go

```go
package filemanager

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

type PDFTextExtractorPlugin struct{}

func (p *PDFTextExtractorPlugin) Process(files []*ManagedFile, fileProcess *FileProcess) ([]*ManagedFile, error) {
	var processedFiles []*ManagedFile

	for _, file := range files {
		if !isPDFFile(file) {
			processedFiles = append(processedFiles, file)
			continue
		}
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "PDFTextExtractor",
			StatusDescription: fmt.Sprintf("Extracting text from PDF: %s", file.FileName),
		}
		fileProcess.AddProcessingUpdate(status)

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
```

## ./filemanager.processing.go

```go
// processing.go
package filemanager

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrRecipeNotFound           = errors.New("recipe not found")
	ErrInvalidMimeType          = errors.New("invalid MIME type")
	ErrInvalidFileSize          = errors.New("invalid file size")
	ErrProcessingPluginNotFound = errors.New("processing plugin not found")
)

type ProcessingPlugin interface {
	Process(files []*ManagedFile, fileProcess *FileProcess) ([]*ManagedFile, error)
}

type ProcessingStep struct {
	PluginName string         `yaml:"pluginName"`
	Params     map[string]any `yaml:"params"`
}

type OutputFormat struct {
	Format          string          `yaml:"format"`
	TargetFileNames []string        `yaml:"targetFileNames"`
	StorageType     FileStorageType `yaml:"storageType"` // public, private, temp
}

type Recipe struct {
	Name              string           `yaml:"name"`
	AcceptedMimeTypes []string         `yaml:"acceptedMimeTypes"`
	MinFileSize       int64            `yaml:"minFileSize"`
	MaxFileSize       int64            `yaml:"maxFileSize"`
	ProcessingSteps   []ProcessingStep `yaml:"processingSteps"`
	OutputFormats     []OutputFormat   `yaml:"outputFormats"`
}

type ProcessingResultFile struct {
	FileName      string
	LocalFilePath string
	URL           string
	FileSize      int64
	MimeType      string
}

type ProcessingStatus struct {
	ProcessID         string
	TimeStamp         int // js timestamp in unix milliseconds
	ProcessorName     string
	StatusDescription string
	Percentage        int
	Error             error
	Done              bool
	ResultingFiles    []ProcessingResultFile
}

func (fm *FileManager) ProcessFile(file *ManagedFile, recipeName string, fileProcess *FileProcess, statusCh chan<- *FileProcess) {
	defer close(statusCh)

	recipe, ok := fm.recipes[recipeName]
	if !ok {
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "RecipeCheck",
			StatusDescription: fmt.Sprintf("Recipe not found: %s", recipeName),
			Error:             fmt.Errorf("recipe not found: %s", recipeName),
			Done:              true,
		}
		fileProcess.AddProcessingUpdate(status)
		statusCh <- fileProcess
		return
	}

	if !isValidMimeType(file.MimeType, recipe.AcceptedMimeTypes) {
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "MimeTypeCheck",
			StatusDescription: fmt.Sprintf("Invalid MIME type: %s", file.MimeType),
			Error:             fmt.Errorf("invalid MIME type: %s", file.MimeType),
			Done:              true,
		}
		fileProcess.AddProcessingUpdate(status)
		statusCh <- fileProcess
		return
	}

	if file.FileSize < recipe.MinFileSize || file.FileSize > recipe.MaxFileSize {
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "FileSizeCheck",
			StatusDescription: fmt.Sprintf("Invalid file size: %d bytes", file.FileSize),
			Error:             fmt.Errorf("invalid file size: %d bytes", file.FileSize),
			Done:              true,
		}
		fileProcess.AddProcessingUpdate(status)
		statusCh <- fileProcess
		return
	}

	files := []*ManagedFile{file}

	for _, step := range recipe.ProcessingSteps {
		plugin, ok := fm.processingPlugins[step.PluginName]
		if !ok {
			status := ProcessingStatus{
				ProcessID:         fileProcess.ID,
				TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
				ProcessorName:     step.PluginName,
				StatusDescription: fmt.Sprintf("Processing plugin not found: %s", step.PluginName),
				Error:             fmt.Errorf("processing plugin not found: %s", step.PluginName),
				Done:              true,
			}
			fileProcess.AddProcessingUpdate(status)
			statusCh <- fileProcess
			return
		}

		processedFiles, err := plugin.Process(files, fileProcess)
		if err != nil {
			status := ProcessingStatus{
				ProcessID:         fileProcess.ID,
				TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
				ProcessorName:     step.PluginName,
				StatusDescription: fmt.Sprintf("Processing failed: %v", err),
				Error:             err,
				Done:              true,
			}
			fileProcess.AddProcessingUpdate(status)
			statusCh <- fileProcess
			return
		}

		files = processedFiles
		percentage := (len(files) * 100) / len(recipe.ProcessingSteps)
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     step.PluginName,
			StatusDescription: fmt.Sprintf("Processing step completed: %s", step.PluginName),
			Percentage:        percentage,
		}
		fileProcess.AddProcessingUpdate(status)
		statusCh <- fileProcess
	}

	var outputFiles []*ManagedFile

	for _, outputFormat := range recipe.OutputFormats {
		for _, targetFileName := range outputFormat.TargetFileNames {
			outputFile := &ManagedFile{
				FileName: targetFileName,
				MetaData: file.MetaData,
			}

			switch outputFormat.StorageType {
			case FileStorageTypePrivate:
				outputFile.LocalFilePath = fm.GetPrivateLocalFilePath(targetFileName)
			case FileStorageTypeTemp:
				outputFile.LocalFilePath = fm.GetLocalTemporaryFilePath(targetFileName)
			case FileStorageTypePublic:
				outputFile.LocalFilePath = fm.GetPublicLocalFilePath(targetFileName)
				outputFile.URL, _ = fm.GetPublicUrlForFile(outputFile.LocalFilePath)
			default:
				status := ProcessingStatus{
					ProcessID:         fileProcess.ID,
					TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
					ProcessorName:     "OutputFormatCheck",
					StatusDescription: fmt.Sprintf("Invalid storage type: %s", outputFormat.StorageType),
					Error:             fmt.Errorf("invalid storage type: %s", outputFormat.StorageType),
					Done:              true,
				}
				fileProcess.AddProcessingUpdate(status)
				statusCh <- fileProcess
				return
			}

			outputFile.Content = file.Content
			outputFile.MimeType = file.MimeType
			outputFile.FileSize = file.FileSize

			err := outputFile.Save()
			if err != nil {
				status := ProcessingStatus{
					ProcessID:         fileProcess.ID,
					TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
					ProcessorName:     "FileSave",
					StatusDescription: fmt.Sprintf("Failed to save output file: %v", err),
					Error:             err,
					Done:              true,
				}
				fileProcess.AddProcessingUpdate(status)
				statusCh <- fileProcess
				return
			}

			outputFiles = append(outputFiles, outputFile)
		}
	}

	var resultingFiles []ProcessingResultFile

	for _, outputFile := range outputFiles {
		resultingFile := ProcessingResultFile{
			FileName:      outputFile.FileName,
			LocalFilePath: outputFile.LocalFilePath,
			URL:           outputFile.URL,
			FileSize:      outputFile.FileSize,
			MimeType:      outputFile.MimeType,
		}
		resultingFiles = append(resultingFiles, resultingFile)
	}

	status := ProcessingStatus{
		ProcessID:         fileProcess.ID,
		TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
		ProcessorName:     "FileProcessing",
		StatusDescription: "File processing completed",
		Percentage:        100,
		Done:              true,
		ResultingFiles:    resultingFiles,
	}
	fileProcess.AddProcessingUpdate(status)
	fileProcess.LatestStatus.Done = true
	statusCh <- fileProcess
}

func isValidMimeType(mimeType string, acceptedMimeTypes []string) bool {
	for _, accepted := range acceptedMimeTypes {
		if mimeType == accepted {
			return true
		}
	}
	return false
}
```

## ./filemanager.models.managedfile.go

```go
package filemanager

import (
	"mime"
	"os"
	"path/filepath"
)

type ManagedFile struct {
	FileName         string         `json:"fileName"`
	MimeType         string         `json:"mimetype"`
	URL              string         `json:"url"`
	LocalFilePath    string         `json:"localFilePath"`
	FileSize         int64          `json:"fileSize"`
	MetaData         map[string]any `json:"metaData"`
	ProcessingErrors []string       `json:"processingErrors"`
	Content          []byte         `json:"-"`
}

func (entity *ManagedFile) GetFileName() string {
	return entity.FileName
}

func (entity *ManagedFile) GetLocalFilePathWithoutFileName() string {
	filepath := filepath.Dir(entity.LocalFilePath)
	return filepath
}

func (entity *ManagedFile) UpdateMimeType() string {
	if entity.LocalFilePath != "" {
		contentType, err := GuessMimeType(entity.LocalFilePath)
		if err != nil {
			return ""
		}
		entity.MimeType = contentType
	}
	return entity.MimeType
}

func (entity *ManagedFile) UpdateFilesize() int64 {
	if entity.FileSize == 0 && entity.LocalFilePath != "" {
		fileInfo, err := os.Stat(entity.LocalFilePath)
		if err != nil {
			return 0
		}
		entity.FileSize = fileInfo.Size()
	}
	return entity.FileSize
}

func (entity *ManagedFile) EnsureFileIsLocal(fm *FileManager, target FileStorageType) (file *ManagedFile, err error) {
	if entity.LocalFilePath == "" || (entity.LocalFilePath != "" && !FileExists(entity.LocalFilePath)) {

		// decide where to download the file to based on the target var and get the respective local path from the FileManager
		localFilePath := fm.GetLocalPathForFile(target, entity.FileName)
		err = DownloadFileFromUrl(entity.URL, localFilePath)
		if err != nil {
			return file, err
		}
		entity.LocalFilePath = localFilePath
		if target == FileStorageTypePublic && entity.URL == "" {
			entity.URL, err = fm.GetPublicUrlForFile(entity.LocalFilePath)
			if err != nil {
				return entity, err
			}
		}
	}
	return entity, nil
}

func (entity *ManagedFile) EnsurePublicURL(fm *FileManager) (pubUrl string, err error) {
	pubUrl = ""
	if entity.URL != "" {
		pubUrl = entity.URL
		return pubUrl, nil
	}
	_, err = entity.EnsureFileIsLocal(fm, FileStorageTypePublic)
	return pubUrl, err
}

func (entity *ManagedFile) SetMetaData(key string, value any) {
	entity.MetaData[key] = value
}

func (entity *ManagedFile) GetMetaData(key string) (value any) {
	val, ok := entity.MetaData[key]
	if ok {
		return val
	}
	return nil
}

func (file *ManagedFile) Save() error {
	// Create the directory if it doesn't exist
	err := os.MkdirAll(filepath.Dir(file.LocalFilePath), os.ModePerm)
	if err != nil {
		return err
	}

	// Open the file for writing
	outputFile, err := os.Create(file.LocalFilePath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// Write the file content to the output file
	_, err = outputFile.Write(file.Content)
	if err != nil {
		return err
	}

	// Update the file metadata
	file.FileSize = int64(len(file.Content))
	file.MimeType = mime.TypeByExtension(filepath.Ext(file.LocalFilePath))

	return nil
}
```


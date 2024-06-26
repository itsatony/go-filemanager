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

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

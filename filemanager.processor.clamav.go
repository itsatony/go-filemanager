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

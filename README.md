# FileManager Package

The FileManager package is a powerful and flexible solution for handling and processing files in Go. It provides a convenient way to manage file storage, retrieval, and processing using a plugin-based architecture.

## Versions

- v0.4.2 fixed public URL generation
- v0.4.1 Added fm.RunProcessingStep helper to run a single processing step instead of a pre-loaded recipe.
- v0.4.0 Added 2 helpers to create ManagedFiles without Processing from  multipart.FileHeader  and another from a local file path.
- v0.3.0 Added support for multiple output files, improved error handling, and enhanced processing status updates with resulting file information.

## Features

- File storage and retrieval: Store and retrieve files from different storage types (public, private, temporary).
- File processing: Process files using various processing plugins, such as image manipulation, PDF manipulation, and more.
- Recipe-based processing: Define processing recipes that specify a sequence of processing steps to be applied to files.
- Upload handling: Handle file uploads and trigger processing recipes based on the uploaded files.

## Usage

### Initialization

To start using the FileManager package, you need to initialize a new instance of the `FileManager` struct:

```go
import "github.com/itsatony/go-filemanager"

fm := filemanager.NewFileManager(publicBasePath, privateBasePath, baseURL, tempPath)
```

- `publicBasePath`: The base path for storing public files.
- `privateBasePath`: The base path for storing private files.
- `baseURL`: The base URL for accessing files.
- `tempPath`: The path for storing temporary files.

### Adding Processing Plugins

To add processing plugins to the FileManager, use the `AddProcessingPlugin` method:

```go
fm.AddProcessingPlugin("image_manipulation", &filemanager.ImageManipulationPlugin{})
fm.AddProcessingPlugin("pdf_manipulation", &filemanager.PDFManipulationPlugin{})
fm.AddProcessingPlugin("pdf_text_extractor", &filemanager.PDFTextExtractorPlugin{})
fm.AddProcessingPlugin("clamav", &filemanager.ClamAVPlugin{})
fm.AddProcessingPlugin("format_converter", &filemanager.FormatConverterPlugin{})
fm.AddProcessingPlugin("exif_metadata_extractor", &filemanager.ExifMetadataExtractorPlugin{})
```

### Loading Recipes

To load processing recipes from a directory, use the `LoadRecipes` method:

```go
err := fm.LoadRecipes("path/to/recipes")
if err != nil {
    // Handle the error
}
```

The recipe files should be in YAML format and stored in the specified directory.

### Processing Files

To process a file using a specific recipe, use the `ProcessFile` method:

```go
fileProcess := filemanager.NewFileProcess("example.jpg", "image_processing_recipe")

statusCh := make(chan *filemanager.FileProcess)
go func() {
    fm.ProcessFile(file, "image_processing_recipe", fileProcess, statusCh)
}()

for processUpdate := range statusCh {
    latestStatus := processUpdate.GetLatestProcessingStatus()
    if latestStatus.Error != nil {
        // Handle the processing error
    } else if latestStatus.Done {
        // Processing completed successfully
    } else {
        // Processing progress update
        fmt.Printf("Processing progress: %d%% - %s\n", latestStatus.Percentage, latestStatus.StatusDescription)
    }
}
```

### Handling File Uploads

To handle file uploads and trigger processing recipes, use the `HandleFileUpload` method:

In this updated example:

We create a statusCh channel to receive the processing status updates, including upload progress updates.
We use a goroutine to handle the file upload asynchronously using the HandleFileUpload function. We pass the fileReader (an io.Reader representing the file data) and the statusCh channel to the function.
If an error occurs during the upload, we handle it appropriately.
After the file is successfully uploaded, we trigger a processing recipe using the ProcessFile function, passing the uploaded file, the recipe name, and the statusCh channel.
We use a for loop to consume the status updates from the statusCh channel.
If the status contains an error, we handle it appropriately.
If the status indicates that the processing is done (status.Done is true), we handle the completion of the processing.
If the status is neither an error nor a completion status, it represents an upload or processing progress update. We print the progress percentage using status.Percentage.

```go
fileProcess := filemanager.NewFileProcess("uploaded_file.pdf", "upload_processing_recipe")

statusCh := make(chan *filemanager.FileProcess)
go func() {
    file, err := fm.HandleFileUpload(fileReader, fileProcess, statusCh)
    if err != nil {
        fmt.Printf("Upload error: %v\n", err)
        return
    }
    
    err = fm.ProcessFile(file, "upload_processing_recipe", fileProcess, statusCh)
    if err != nil {
        fmt.Printf("Processing error: %v\n", err)
    }
}()

for processUpdate := range statusCh {
    latestStatus := processUpdate.LatestStatus
    if latestStatus.Error != nil {
        fmt.Printf("Processing error: %v\n", latestStatus.Error)
    } else if latestStatus.Done {
        fmt.Printf("Processing completed successfully\n")
        for _, resultingFile := range latestStatus.ResultingFiles {
            fmt.Printf("Resulting file: %s\n", resultingFile.FileName)
            fmt.Printf("  Local file path: %s\n", resultingFile.LocalFilePath)
            fmt.Printf("  URL: %s\n", resultingFile.URL)
            fmt.Printf("  File size: %d bytes\n", resultingFile.FileSize)
            fmt.Printf("  MIME type: %s\n", resultingFile.MimeType)
        }
    } else {
        fmt.Printf("Progress: %d%% - %s\n", latestStatus.Percentage, latestStatus.StatusDescription)
    }
}
```

## Example Recipes

Here are a few example recipes that demonstrate the usage of different processing plugins:

### Image Processing Recipe

```yaml
name: image_processing_recipe
accepted_mime_types:
  - image/jpeg
  - image/png
min_file_size: 1024
max_file_size: 10485760
processing_steps:
  - plugin_name: image_manipulation
    params:
      format: webp
      width: 800
      height: 600
      aspect_ratio: "4:3"
output_formats:
  - format: webp
    target_file_name: processed_image.webp
    storage_type: public
```

This recipe processes image files by converting them to WebP format, resizing them to 800x600 pixels, and cropping them to a 4:3 aspect ratio. The processed image is stored as a public file.

### PDF Text Extraction Recipe

```yaml
name: pdf_text_extraction_recipe
accepted_mime_types:
  - application/pdf
min_file_size: 1024
max_file_size: 52428800
processing_steps:
  - plugin_name: pdf_text_extractor
    params:
      output_format: markdown
output_formats:
  - format: md
    target_file_name: extracted_text.md
    storage_type: private
```

This recipe extracts text from PDF files and converts it to Markdown format. The extracted text is stored as a private file.

### Upload Processing Recipe

```yaml
name: upload_processing_recipe
accepted_mime_types:
  - image/jpeg
  - image/png
  - application/pdf
min_file_size: 1024
max_file_size: 52428800
processing_steps:
  - plugin_name: image_manipulation
    params:
      format: jpg
      width: 1200
      height: 800
  - plugin_name: pdf_manipulation
    params:
      manipulation_type: compress
      compression_level: medium
output_formats:
  - format: jpg
    target_file_name: processed_upload.jpg
    storage_type: public
  - format: pdf
    target_file_name: compressed_upload.pdf
    storage_type: private
```

This recipe processes uploaded files based on their MIME type. If the uploaded file is an image, it is converted to JPEG format and resized to 1200x800 pixels. If the uploaded file is a PDF, it is compressed using the medium compression level. The processed files are stored as public (for images) and private (for PDFs) files.

## Included Plugins / Processors

The FileManager package comes with several built-in plugins and processors that can be used to manipulate and process files. Here's a list and description of the plugins and processors available:

### Image Manipulation Plugin

The Image Manipulation plugin allows you to perform various image processing operations on image files. It supports the following parameters:

- `format`: The output format of the processed image. Supported formats: "jpg", "png", "webp".
- `width`: The desired width of the processed image in pixels.
- `height`: The desired height of the processed image in pixels.
- `aspect_ratio`: The desired aspect ratio of the processed image. Supported aspect ratios: "1:1", "4:3", "16:9", "21:9".

This plugin can be used to resize, crop, and convert image files to different formats.

### PDF Text Extractor Plugin

The PDF Text Extractor plugin allows you to extract text from PDF files and convert it to plain text or Markdown format. It supports the following parameter:

- `output_format`: The output format of the extracted text. Supported formats: "text", "markdown".

This plugin is useful for extracting text content from PDF files and converting it to a more readable and editable format.

### PDF Manipulation Plugin

The PDF Manipulation plugin allows you to perform various operations on PDF files, such as extracting pages, merging PDFs, compressing PDFs, and reordering pages. It supports the following parameters:

- `manipulation_type`: The type of manipulation to perform on the PDF. Supported types: "extract", "merge", "compress", "reorder".
- `start_page` (for "extract"): The starting page number to extract (inclusive).
- `end_page` (for "extract"): The ending page number to extract (inclusive).
- `merge_files` (for "merge"): An array of file names to be merged with the base PDF.
- `compression_level` (for "compress"): The compression level to apply. Supported levels: "low", "medium", "high".
- `page_order` (for "reorder"): An array of page numbers representing the desired order of pages.

This plugin provides powerful capabilities for manipulating PDF files, such as extracting specific pages, merging multiple PDFs into one, compressing PDFs to reduce file size, and reordering pages.

### ClamAV Plugin

The ClamAV plugin allows you to scan files for viruses using the ClamAV antivirus engine. It doesn't require any additional parameters.

This plugin is useful for ensuring the security of uploaded files by scanning them for known viruses and malware using the ClamAV engine.

These plugins and processors can be used individually or chained together in processing recipes to create custom file processing workflows. The FileManager package provides flexibility and extensibility, allowing you to easily add new plugins and processors to meet your specific requirements.

For detailed information on how to use these plugins and processors, please refer to the documentation and examples provided in the README file.

Here's a section for the Format Converter Processor plugin that you can add to the README file, along with an examples section:

## Format Converter Processor Plugin

The Format Converter Processor plugin allows you to convert various file formats into text-based versions suitable for further processing or injection into Large Language Models (LLMs). It currently supports the following file format conversions:

- DOCX to plain text
- DOCX to Markdown
- Excel (XLS, XLSX) to CSV

The plugin uses the following libraries for file format conversions:

- `github.com/yuin/goldmark` for DOCX to Markdown conversion
- `github.com/360EntSecGroup-Skylar/excelize/v2` for Excel to CSV conversion

### Limitations

- The DOCX to plain text conversion is currently a placeholder implementation that assumes the content is already in plain text format. You may need to replace it with a custom implementation or a library that converts DOCX to plain text.
- The Excel to CSV conversion currently converts only the first sheet of the Excel file. If you need to handle multiple sheets or specify a specific sheet, you may need to modify the `convertExcelToCSV` function accordingly.

Please refer to the plugin's source code for more details on its implementation and functionality.

## Exif Metadata Extractor Plugin

The Exif Metadata Extractor plugin allows you to extract Exif metadata from image files. It retrieves information such as camera make, model, capture date and time, GPS coordinates, focal length, aperture, exposure time, and ISO speed ratings.

The plugin uses the following library for Exif metadata extraction:

- `github.com/rwcarlsen/goexif/exif` for extracting Exif metadata from image files

### Exif Metadata Extractor Usage

To use the Exif Metadata Extractor plugin, include it in your processing pipeline by adding the following configuration to your recipe:

```yaml
processing_steps:
  - plugin_name: exif_metadata_extractor
```

The plugin will automatically detect image files based on their MIME type and extract the Exif metadata.

## Versions

- v0.2.0: updated processUpdates to include more information.
- v0.1.2: Minor updates and improvements to the FileManager package. Added a processor plugin for file-format conversion of .docx and .xlsx to text, markdown and csv. added exif metadata extraction processor.
- v0.1.0: Initial release with basic file storage and retrieval functionality. File Upload handling and recipe-based processing with a few example recipes and processor plugins.

## Installation

To use the FileManager package in your Go project, you need to install it using the following command:

```bash
go get github.com/itsatony/go-filemanager
```

## Contributing

Contributions to the FileManager package are welcome! If you find any issues or have suggestions for improvements, please open an issue or submit a pull request on the GitHub repository.

## License

The FileManager package is open-source software licensed under the [MIT License](LICENSE).

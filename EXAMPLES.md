# FileManager Package Documentation

The FileManager package provides a powerful and flexible solution for handling and processing files in Go. It offers a range of processing plugins that can be used individually or chained together to achieve complex file processing workflows.

## Processing Plugins

### Image Manipulation Plugin

The Image Manipulation plugin allows you to perform various image processing operations on image files. It supports the following parameters:

- `format`: The output format of the processed image. Supported formats: "jpg", "png", "webp".
- `width`: The desired width of the processed image in pixels.
- `height`: The desired height of the processed image in pixels.
- `aspect_ratio`: The desired aspect ratio of the processed image. Supported aspect ratios: "1:1", "4:3", "16:9", "21:9".

Example usage:

```yaml
processing_steps:
  - plugin_name: image_manipulation
    params:
      format: webp
      width: 800
      height: 600
      aspect_ratio: "4:3"
```

### PDF Text Extractor Plugin

The PDF Text Extractor plugin allows you to extract text from PDF files and convert it to plain text or Markdown format. It supports the following parameter:

- `output_format`: The output format of the extracted text. Supported formats: "text", "markdown".

Example usage:

```yaml
processing_steps:
  - plugin_name: pdf_text_extractor
    params:
      output_format: markdown
```

### PDF Manipulation Plugin

The PDF Manipulation plugin allows you to perform various operations on PDF files, such as extracting pages, merging PDFs, compressing PDFs, and reordering pages. It supports the following parameters:

- `manipulation_type`: The type of manipulation to perform on the PDF. Supported types: "extract", "merge", "compress", "reorder".
- `start_page` (for "extract"): The starting page number to extract (inclusive).
- `end_page` (for "extract"): The ending page number to extract (inclusive).
- `merge_files` (for "merge"): An array of file names to be merged with the base PDF.
- `compression_level` (for "compress"): The compression level to apply. Supported levels: "low", "medium", "high".
- `page_order` (for "reorder"): An array of page numbers representing the desired order of pages.

Example usage:

```yaml
processing_steps:
  - plugin_name: pdf_manipulation
    params:
      manipulation_type: extract
      start_page: 1
      end_page: 3
  - plugin_name: pdf_manipulation
    params:
      manipulation_type: merge
      merge_files:
        - file1.pdf
        - file2.pdf
  - plugin_name: pdf_manipulation
    params:
      manipulation_type: compress
      compression_level: medium
  - plugin_name: pdf_manipulation
    params:
      manipulation_type: reorder
      page_order: [3, 1, 2]
```

### ClamAV Plugin

The ClamAV plugin allows you to scan files for viruses using the ClamAV antivirus engine. It doesn't require any additional parameters.

Example usage:

```yaml
processing_steps:
  - plugin_name: clamav
```

## Format Converter Processor Plugin

The Format Converter Processor plugin allows you to convert various file formats into text-based versions suitable for further processing or injection into Large Language Models (LLMs). It currently supports the following file format conversions:

- DOCX to plain text
- DOCX to Markdown
- Excel (XLS, XLSX) to CSV

### Usage

To use the Format Converter Processor plugin, include it in your processing pipeline by adding the following configuration to your recipe:

```yaml
processing_steps:
  - plugin_name: format_converter
```

The plugin will automatically detect the file format based on the MIME type and apply the appropriate conversion.

### Examples

#### Converting DOCX to Plain Text

To convert a DOCX file to plain text, simply include the DOCX file in the processing pipeline:

```yaml
name: docx_to_text_recipe
accepted_mime_types:
  - application/vnd.openxmlformats-officedocument.wordprocessingml.document
processing_steps:
  - plugin_name: format_converter
output_formats:
  - format: txt
    target_file_name: converted_text.txt
    storage_type: private
```

The plugin will attempt to convert the DOCX file to plain text. If the conversion fails, it will fall back to converting the DOCX file to Markdown.

#### Converting DOCX to Markdown

To convert a DOCX file to Markdown, include the DOCX file in the processing pipeline:

```yaml
name: docx_to_markdown_recipe
accepted_mime_types:
  - application/vnd.openxmlformats-officedocument.wordprocessingml.document
processing_steps:
  - plugin_name: format_converter
output_formats:
  - format: md
    target_file_name: converted_markdown.md
    storage_type: private
```

The plugin will convert the DOCX file to Markdown format using the `goldmark` library.

#### Converting Excel to CSV

To convert an Excel file (XLS or XLSX) to CSV, include the Excel file in the processing pipeline:

```yaml
name: excel_to_csv_recipe
accepted_mime_types:
  - application/vnd.ms-excel
  - application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
processing_steps:
  - plugin_name: format_converter
output_formats:
  - format: csv
    target_file_name: converted_csv.csv
    storage_type: private
```

The plugin will convert the Excel file to CSV format using the `excelize` library. Note that the plugin currently converts only the first sheet of the Excel file to CSV.

### Limitations

- The DOCX to plain text conversion is currently a placeholder implementation that assumes the content is already in plain text format. You may need to replace it with a custom implementation or a library that converts DOCX to plain text.
- The Excel to CSV conversion currently converts only the first sheet of the Excel file. If you need to handle multiple sheets or specify a specific sheet, you may need to modify the `convertExcelToCSV` function accordingly.

Please refer to the plugin's source code for more details on its implementation and functionality.

## Exif Metadata Extractor Plugin

The Exif Metadata Extractor plugin allows you to extract Exif metadata from image files. It retrieves information such as camera make, model, capture date and time, GPS coordinates, focal length, aperture, exposure time, and ISO speed ratings.

### Exif Metadata Extractor Usage

To use the Exif Metadata Extractor plugin, include it in your processing pipeline by adding the following configuration to your recipe:

```yaml
processing_steps:
  - plugin_name: exif_metadata_extractor
```

The plugin will automatically detect image files based on their MIME type and extract the Exif metadata.

### Exif Metadata Extractor Examples

#### Extracting Exif Metadata from Image Files

To extract Exif metadata from image files, include the image files in the processing pipeline:

```yaml
name: exif_metadata_extraction_recipe
accepted_mime_types:
  - image/jpeg
  - image/png
processing_steps:
  - plugin_name: exif_metadata_extractor
```

The plugin will extract the available Exif metadata from the image files and store it in the `MetaData` field of the `ManagedFile` object under the key "exif". The extracted metadata will be in the form of a map, where the keys are the Exif field names and the values are the corresponding metadata values.

### Exif Metadata Extractor Limitations

- The plugin relies on the `github.com/rwcarlsen/goexif/exif` library for Exif metadata extraction. It may not support all possible Exif fields or handle all variations of Exif data.
- If an image file doesn't contain Exif metadata or if certain Exif fields are not available, the corresponding entries in the extracted metadata map will be missing.

Please refer to the plugin's source code for more details on its implementation and functionality.

## Chained Processing

The FileManager package allows you to chain multiple processing plugins together to create complex file processing workflows. You can define a sequence of processing steps in a recipe, and the FileManager will execute them in the specified order.

Example of chained processing:

```yaml
name: chained_processing_recipe
accepted_mime_types:
  - application/pdf
min_file_size: 1024
max_file_size: 52428800
processing_steps:
  - plugin_name: clamav
  - plugin_name: pdf_manipulation
    params:
      manipulation_type: extract
      start_page: 1
      end_page: 3
  - plugin_name: pdf_text_extractor
    params:
      output_format: markdown
  - plugin_name: pdf_manipulation
    params:
      manipulation_type: compress
      compression_level: high
output_formats:
  - format: md
    target_file_name: extracted_text.md
    storage_type: private
  - format: pdf
    target_file_name: compressed_extract.pdf
    storage_type: public
```

In this example, the chained processing recipe performs the following steps:

1. Scan the input PDF file for viruses using the ClamAV plugin.
2. Extract pages 1 to 3 from the PDF using the PDF Manipulation plugin.
3. Extract the text from the extracted pages and convert it to Markdown format using the PDF Text Extractor plugin.
4. Compress the extracted pages using the PDF Manipulation plugin with high compression level.
5. Store the extracted text as a private Markdown file and the compressed PDF as a public file.

This is just one example of how you can chain multiple processing plugins together to achieve complex file processing workflows. You can customize the processing steps and their parameters based on your specific requirements.

v## Conclusion

The FileManager package provides a flexible and extensible framework for handling and processing files in Go. With its plugin-based architecture and support for chained processing, you can easily create custom file processing workflows tailored to your needs.

For more detailed information on the FileManager package, including installation instructions, usage examples, and contributing guidelines, please refer to the main README file.

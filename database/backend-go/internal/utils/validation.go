package utils

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// ValidateFile checks the file magic number to ensure it's a valid Excel or CSV file
func ValidateFile(file multipart.File) error {
	// Read the first 512 bytes to detect content type
	buffer := make([]byte, 512)
	_, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return err
	}

	// Reset file pointer
	file.Seek(0, 0)

	contentType := http.DetectContentType(buffer)

	// Magic numbers/signatures
	// ZIP (xlsx is a zip): PK..
	isZip := bytes.HasPrefix(buffer, []byte{0x50, 0x4B, 0x03, 0x04})

	// OLE2 (xls): D0 CF 11 E0
	isOle := bytes.HasPrefix(buffer, []byte{0xD0, 0xCF, 0x11, 0xE0})

	// CSV is harder as it's plain text, but we can check if it looks like text
	isText := contentType == "text/plain" ||
		contentType == "text/csv" ||
		contentType == "application/csv" ||
		contentType == "application/vnd.ms-excel" ||
		contentType == "application/octet-stream" ||
		strings.HasPrefix(contentType, "text/") ||
		isLikelyText(buffer)

	if isZip || isOle || isText {
		return nil
	}

	return errors.New("invalid file signature: not an Excel or CSV file")
}

// isLikelyText checks if the buffer is predominantly printable/textual data.
func isLikelyText(buf []byte) bool {
	if len(buf) == 0 {
		return false
	}
	printable := 0
	for _, b := range buf {
		if b == 0 {
			// NUL bytes usually indicate binary
			return false
		}
		if (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t' {
			printable++
		}
	}
	ratio := float64(printable) / float64(len(buf))
	return ratio >= 0.75
}

// ValidatePassword checks password validity with minimal rules.
func ValidatePassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return errors.New("password cannot be empty")
	}
	return nil
}

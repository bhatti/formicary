package artifacts

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// ZipFiles zip files — entries containing glob characters (* ? [) are expanded
// via filepath.Glob; glob patterns that match nothing are silently skipped.
func ZipFiles(newZipFile *os.File, files []string) (err error) {
	zipWriter := zip.NewWriter(newZipFile)
	defer func() {
		_ = zipWriter.Close()
	}()

	for _, pattern := range files {
		if isGlob(pattern) {
			matches, globErr := filepath.Glob(pattern)
			if globErr != nil {
				return fmt.Errorf("invalid glob pattern %q: %w", pattern, globErr)
			}
			for _, file := range matches {
				if err = addFileToZip(zipWriter, file); err != nil {
					return err
				}
			}
		} else {
			if err = addFileToZip(zipWriter, pattern); err != nil {
				return err
			}
		}
	}
	return nil
}

func isGlob(pattern string) bool {
	for _, ch := range pattern {
		if ch == '*' || ch == '?' || ch == '[' {
			return true
		}
	}
	return false
}

// UnzipFile unzips file into destination directory
func UnzipFile(zipFile string, destDir string) (err error) {
	r, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = r.Close()
	}()
	_ = os.MkdirAll(destDir, 0750)

	for _, f := range r.File {
		if err := extractFile(f, destDir); err != nil {
			return err
		}
	}

	return nil
}

func extractFile(f *zip.File, destDir string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer ioutil.NopCloser(rc)

	path := filepath.Join(destDir, f.Name)

	if f.FileInfo().IsDir() {
		_ = os.MkdirAll(path, f.Mode())
	} else {
		_ = os.MkdirAll(filepath.Dir(path), f.Mode())
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		defer ioutil.NopCloser(f)

		_, err = io.Copy(f, rc) // Potential DoS vulnerability via decompression bomb
		if err != nil {
			return err
		}
	}

	return nil
}

func addFileToZip(zipWriter *zip.Writer, filename string) error {
	fileToZip, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer ioutil.NopCloser(fileToZip)

	info, err := fileToZip.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = filename
	header.Method = zip.Deflate
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, fileToZip)
	return err
}

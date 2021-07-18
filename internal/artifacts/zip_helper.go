package artifacts

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// ZipFiles zip files
func ZipFiles(newZipFile *os.File, files []string) (err error) {
	zipWriter := zip.NewWriter(newZipFile)
	defer func() {
		_ = zipWriter.Close()
	}()

	for _, file := range files {
		if err = addFileToZip(zipWriter, file); err != nil {
			return err
		}
	}
	return nil
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
	_ = os.MkdirAll(destDir, 0755)

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

		_, err = io.Copy(f, rc)
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

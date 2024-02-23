package trace

import (
	"bufio"
	"bytes"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"plexobject.com/formicary/internal/utils"
	"sync"

	"github.com/markelog/trie"
)

const maskedText = "[****]"
const defaultBytesLimit = 4 * 1024 * 1024 // 4MB

// LineFeeder - callback function when new-line is reached
type LineFeeder func([]byte, string)

// Buffer for tracing
type Buffer struct {
	lineFeeder    LineFeeder
	writer        io.WriteCloser
	tagsLock      sync.RWMutex
	bufferLock    sync.RWMutex
	logFile       *os.File
	logSize       int
	logWriter     *bufio.Writer
	advanceBuffer bytes.Buffer
	lineBuffer    bytes.Buffer
	bytesLimit    int
	tags          string
	finish        chan struct{}
	checksum      hash.Hash32
	maskTree      *trie.Trie
}

// New constructor
func New(lineFeeder LineFeeder) (*Buffer, error) {
	logFile, err := ioutil.TempFile("", "trace")
	if err != nil {
		return nil, err
	}

	checksum := crc32.NewIEEE()

	reader, writer := io.Pipe()
	buffer := &Buffer{
		lineFeeder: lineFeeder,
		writer:     writer,
		bytesLimit: defaultBytesLimit,
		finish:     make(chan struct{}),
		logFile:    logFile,
		checksum:   checksum,
		logWriter:  bufio.NewWriter(io.MultiWriter(logFile, checksum)),
	}
	go buffer.process(reader)
	return buffer, nil
}

// SetMasked for masking secured fields
func (b *Buffer) SetMasked(values []string) {
	if len(values) == 0 {
		b.maskTree = nil
		return
	}

	maskTree := trie.New()
	for _, value := range values {
		maskTree.Add(value, nil)
	}
	b.maskTree = maskTree
}

// SetLimit limit max size
func (b *Buffer) SetLimit(size int) {
	b.bufferLock.Lock()
	b.bytesLimit = size
	b.bufferLock.Unlock()
}

// Size size of buffer
func (b *Buffer) Size() int {
	return b.logSize
}

// Reader reads bytes
func (b *Buffer) Reader(offset, n int) (io.ReadSeeker, error) {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()

	err := b.logWriter.Flush()
	if err != nil {
		return nil, err
	}

	return io.NewSectionReader(b.logFile, int64(offset), int64(n)), nil
}

// Bytes from buffer
func (b *Buffer) Bytes(offset, n int) ([]byte, error) {
	reader, err := b.Reader(offset, n)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(reader)
}

// Write adds bytes
func (b *Buffer) Write(data []byte, tags string) (n int, err error) {
	b.setTags(tags)
	n, err = b.writer.Write(data)
	return
}

// Finish closes buffer
func (b *Buffer) Finish() {
	// wait for trace to finish
	_ = b.writer.Close()
	<-b.finish
}

// Close closes buffer
func (b *Buffer) Close() {
	_ = b.logFile.Close()
	_ = os.Remove(b.logFile.Name())
}

// Checksum sum of buffer
func (b *Buffer) Checksum() string {
	return fmt.Sprintf("crc32:%08x", b.checksum.Sum32())
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (b *Buffer) advanceAllUnsafe() error {
	b.lineBuffer.Write(b.advanceBuffer.Bytes())
	tags := b.getTags()
	if b.lineBuffer.Len() > 1 && b.lineBuffer.Bytes()[b.lineBuffer.Len()-2] == '\r' &&
		b.lineBuffer.Bytes()[b.lineBuffer.Len()-1] == '\n' {
		b.lineFeeder(b.lineBuffer.Bytes(), tags)
		b.lineBuffer.Reset()
	}
	n, err := b.advanceBuffer.WriteTo(b.logWriter)
	b.logSize += int(n)
	return err
}

func (b *Buffer) getTags() string {
	b.tagsLock.RLock()
	defer b.tagsLock.RUnlock()
	return b.tags
}

func (b *Buffer) setTags(tags string) {
	b.tagsLock.Lock()
	defer b.tagsLock.Unlock()
	b.tags = tags
}

func (b *Buffer) advanceAll() {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()

	_ = b.advanceAllUnsafe()
}

// advanceLogUnsafe is assumed to be run every character
func (b *Buffer) advanceLogUnsafe() error {
	// advance all if no masking is enabled
	if b.maskTree == nil {
		return b.advanceAllUnsafe()
	}

	rest := b.advanceBuffer.String()
	results := b.maskTree.Search(rest)
	if len(results) == 0 {
		// we can advance as no match was found
		return b.advanceAllUnsafe()
	}

	// full match was found
	if len(results) == 1 && results[0].Key == rest {
		b.advanceBuffer.Reset()
		b.advanceBuffer.WriteString(maskedText)
		return b.advanceAllUnsafe()
	}

	// partial match, wait for more characters
	return nil
}

func (b *Buffer) limitExceededMessage() string {
	return fmt.Sprintf(
		"\n%sJob's log exceeded limit of %v bytes.%s\n",
		utils.AnsiBoldRed,
		b.bytesLimit,
		utils.AnsiReset,
	)
}

func (b *Buffer) writeRune(r rune) error {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()

	// over trace limit
	if b.logSize > b.bytesLimit {
		return io.EOF
	}

	if _, err := b.advanceBuffer.WriteRune(r); err != nil {
		return err
	}

	if err := b.advanceLogUnsafe(); err != nil {
		return err
	}

	// under trace limit
	if b.logSize <= b.bytesLimit {
		return nil
	}

	b.advanceBuffer.Reset()
	b.advanceBuffer.WriteString(b.limitExceededMessage())
	return b.advanceAllUnsafe()
}

func (b *Buffer) process(pipe *io.PipeReader) {
	defer func() { _ = pipe.Close() }()

	reader := bufio.NewReader(pipe)

	for {
		r, s, err := reader.ReadRune()
		if s <= 0 {
			break
		}

		if err == nil {
			// only write valid characters
			_ = b.writeRune(r)
		}
	}

	b.advanceAll()
	close(b.finish)
}

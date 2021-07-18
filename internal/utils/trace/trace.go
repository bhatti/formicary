package trace

import (
	"github.com/sirupsen/logrus"
)

// JobTraceImpl --
type JobTraceImpl struct {
	buffer *Buffer
}

// JobTrace interface
type JobTrace interface {
	Writeln(input string) (n int, err error)
	Write(data []byte) (n int, err error)
	Finish() ([]byte, error)
	Close()
}

// NewJobTrace --
func NewJobTrace(lineFeeder LineFeeder, bufferLimit int, mask []string) (JobTrace, error) {
	buffer, err := New(lineFeeder)
	if err != nil {
		return nil, err
	}
	buffer.SetMasked(mask)

	buffer.SetLimit(bufferLimit)

	return &JobTraceImpl{
		buffer: buffer,
	}, nil
}

// Writeln writes string and new-line to the Buffer --
func (j *JobTraceImpl) Writeln(input string) (n int, err error) {
	n, err = j.Write([]byte(input))
	//if err == nil {
	//	_, err = j.Write([]byte("\r\n"))
	//}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.Debug(input)
	}
	return
}

// Write Data to the Buffer
func (j *JobTraceImpl) Write(data []byte) (n int, err error) {
	n, err = j.buffer.Write(data)
	if err == nil {
		_, err = j.buffer.Write([]byte("\r\n"))
	}
	return
}

// Finish closes writer and returns contents
func (j *JobTraceImpl) Finish() ([]byte, error) {
	j.buffer.Finish()
	return j.buffer.Bytes(0, j.buffer.Size())
}

// Close closes buffer
func (j *JobTraceImpl) Close() {
	j.buffer.Close()
}

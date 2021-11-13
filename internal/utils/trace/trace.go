package trace

// JobTraceImpl --
type JobTraceImpl struct {
	buffer *Buffer
}

// JobTrace interface
type JobTrace interface {
	Write(data []byte, tags string) (n int, err error)
	Writeln(data string, tags string) (n int, err error)
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

// Writeln Data to the Buffer
func (j *JobTraceImpl) Writeln(data string, tags string) (n int, err error) {
	return j.Write([]byte(data), tags)
}

// Write Data to the Buffer
func (j *JobTraceImpl) Write(data []byte, tags string) (n int, err error) {
	n, err = j.buffer.Write(append(data, []byte("\r\n")...), tags)
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

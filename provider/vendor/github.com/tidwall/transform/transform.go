// Package transform provides a convenient utility for transforming one data
// format to another.
package transform

// Transformer represents a transform reader.
type Transformer struct {
	tfn func() ([]byte, error) // user-defined transform function
	buf []byte                 // read buffer
	idx int                    // read buffer index
	err error                  // last error
}

// NewTransformer returns an object that can be used for transforming one
// data formant to another. The param is a function that performs the
// conversion and returns the transformed data in chunks/messages.
func NewTransformer(fn func() ([]byte, error)) *Transformer {
	return &Transformer{tfn: fn}
}

// ReadMessage allows for reading a one transformed message at a time.
func (r *Transformer) ReadMessage() ([]byte, error) {
	return r.tfn()
}

// Read conforms to io.Reader
func (r *Transformer) Read(p []byte) (n int, err error) {
	if len(r.buf)-r.idx > 0 {
		// There's data in the read buffer, return it prior to returning errors
		// or reading more messages.
		if len(r.buf)-r.idx > len(p) {
			// The input slice is smaller than the read buffer, copy a subslice
			// of the read buffer and increase the read index.
			copy(p, r.buf[r.idx:r.idx+len(p)])
			r.idx += len(p)
			return len(p), nil
		}
		// Copy the entire read buffer to the input slice.
		n = len(r.buf) - r.idx
		copy(p[:n], r.buf[r.idx:])
		r.buf = r.buf[:0] // reset the read buffer, keeping it's capacity
		r.idx = 0         // rewind the read buffer index
		return n, nil
	}
	if r.err != nil {
		return 0, r.err
	}
	var msg []byte
	msg, r.err = r.ReadMessage()
	// We should immediately append the incoming message to the read
	// buffer to allow for the implemented transformer to repurpose
	// it's own message space if needed.
	r.buf = append(r.buf, msg...)
	return r.Read(p)
}

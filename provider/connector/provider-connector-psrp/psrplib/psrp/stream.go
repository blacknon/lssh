package psrp

import "fmt"

type MessageStreamDecoder struct {
	buffer  []byte
	pending map[uint64][]Fragment
	order   []uint64
}

func (d *MessageStreamDecoder) Push(raw []byte) ([]Message, error) {
	d.buffer = append(d.buffer, raw...)
	if d.pending == nil {
		d.pending = map[uint64][]Fragment{}
	}

	messages := []Message{}
	for {
		if len(d.buffer) < fragmentHeaderLength {
			break
		}

		blobLength := int(readFragmentBlobLength(d.buffer))
		totalLength := fragmentHeaderLength + blobLength
		if len(d.buffer) < totalLength {
			break
		}

		fragment, err := DecodeFragment(d.buffer[:totalLength])
		if err != nil {
			return nil, err
		}
		d.buffer = d.buffer[totalLength:]

		if _, ok := d.pending[fragment.ObjectID]; !ok {
			d.order = append(d.order, fragment.ObjectID)
		}
		d.pending[fragment.ObjectID] = append(d.pending[fragment.ObjectID], fragment)

		if !fragment.End {
			continue
		}

		payload, err := AssembleFragments(d.pending[fragment.ObjectID])
		if err != nil {
			return nil, err
		}
		message, err := DecodeMessage(payload)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
		delete(d.pending, fragment.ObjectID)
		d.dropOrder(fragment.ObjectID)
	}

	return messages, nil
}

func (d *MessageStreamDecoder) Remaining() []byte {
	return append([]byte(nil), d.buffer...)
}

func (d *MessageStreamDecoder) Finalize() error {
	if len(d.buffer) > 0 {
		return fmt.Errorf("psrp stream has %d trailing bytes", len(d.buffer))
	}
	if len(d.pending) > 0 {
		return fmt.Errorf("psrp stream has %d incomplete messages", len(d.pending))
	}
	return nil
}

func (d *MessageStreamDecoder) dropOrder(objectID uint64) {
	for i, id := range d.order {
		if id != objectID {
			continue
		}
		d.order = append(d.order[:i], d.order[i+1:]...)
		return
	}
}

func readFragmentBlobLength(raw []byte) uint32 {
	return uint32(raw[17])<<24 | uint32(raw[18])<<16 | uint32(raw[19])<<8 | uint32(raw[20])
}

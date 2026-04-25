package psrp

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"sort"
)

const (
	fragmentHeaderLength = 21
	maxFragmentBlobSize  = 32768
)

type Fragment struct {
	ObjectID   uint64
	FragmentID uint64
	Start      bool
	End        bool
	Blob       []byte
}

func EncodeFragment(fragment Fragment) []byte {
	buf := make([]byte, fragmentHeaderLength+len(fragment.Blob))
	binary.BigEndian.PutUint64(buf[0:8], fragment.ObjectID)
	binary.BigEndian.PutUint64(buf[8:16], fragment.FragmentID)

	var flags byte
	if fragment.End {
		flags |= 0x02
	}
	if fragment.Start {
		flags |= 0x01
	}
	buf[16] = flags
	binary.BigEndian.PutUint32(buf[17:21], uint32(len(fragment.Blob)))
	copy(buf[21:], fragment.Blob)
	return buf
}

func DecodeFragment(raw []byte) (Fragment, error) {
	if len(raw) < fragmentHeaderLength {
		return Fragment{}, fmt.Errorf("psrp fragment too short: %d", len(raw))
	}

	blobLength := binary.BigEndian.Uint32(raw[17:21])
	if len(raw) < fragmentHeaderLength+int(blobLength) {
		return Fragment{}, fmt.Errorf("psrp fragment blob truncated: have %d want %d", len(raw)-fragmentHeaderLength, blobLength)
	}

	flags := raw[16]
	return Fragment{
		ObjectID:   binary.BigEndian.Uint64(raw[0:8]),
		FragmentID: binary.BigEndian.Uint64(raw[8:16]),
		Start:      flags&0x01 != 0,
		End:        flags&0x02 != 0,
		Blob:       append([]byte(nil), raw[21:21+int(blobLength)]...),
	}, nil
}

func DecodeFragments(raw []byte) ([]Fragment, error) {
	fragments := []Fragment{}
	for offset := 0; offset < len(raw); {
		if len(raw[offset:]) < fragmentHeaderLength {
			return nil, fmt.Errorf("psrp fragment stream too short at offset %d", offset)
		}

		blobLength := binary.BigEndian.Uint32(raw[offset+17 : offset+21])
		end := offset + fragmentHeaderLength + int(blobLength)
		if end > len(raw) {
			return nil, fmt.Errorf("psrp fragment stream truncated at offset %d", offset)
		}

		fragment, err := DecodeFragment(raw[offset:end])
		if err != nil {
			return nil, err
		}
		fragments = append(fragments, fragment)
		offset = end
	}

	return fragments, nil
}

func FragmentMessage(objectID uint64, message []byte, maxBlobSize int) []Fragment {
	if maxBlobSize <= 0 || maxBlobSize > maxFragmentBlobSize {
		maxBlobSize = maxFragmentBlobSize
	}

	if len(message) == 0 {
		return []Fragment{{
			ObjectID:   objectID,
			FragmentID: 0,
			Start:      true,
			End:        true,
			Blob:       nil,
		}}
	}

	fragments := []Fragment{}
	for offset, fragmentID := 0, uint64(0); offset < len(message); offset, fragmentID = offset+maxBlobSize, fragmentID+1 {
		end := offset + maxBlobSize
		if end > len(message) {
			end = len(message)
		}
		fragments = append(fragments, Fragment{
			ObjectID:   objectID,
			FragmentID: fragmentID,
			Start:      offset == 0,
			End:        end == len(message),
			Blob:       append([]byte(nil), message[offset:end]...),
		})
	}
	return fragments
}

func AssembleFragments(fragments []Fragment) ([]byte, error) {
	if len(fragments) == 0 {
		return nil, nil
	}

	sorted := append([]Fragment(nil), fragments...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ObjectID != sorted[j].ObjectID {
			return sorted[i].ObjectID < sorted[j].ObjectID
		}
		return sorted[i].FragmentID < sorted[j].FragmentID
	})

	first := sorted[0]
	if !first.Start || first.FragmentID != 0 {
		return nil, fmt.Errorf("psrp fragments do not start with a Start fragment")
	}

	var payload bytes.Buffer
	expectedFragmentID := uint64(0)
	for _, fragment := range sorted {
		if fragment.ObjectID != first.ObjectID {
			return nil, fmt.Errorf("psrp fragments contain multiple object IDs")
		}
		if fragment.FragmentID != expectedFragmentID {
			return nil, fmt.Errorf("psrp fragment sequence gap: got %d want %d", fragment.FragmentID, expectedFragmentID)
		}
		payload.Write(fragment.Blob)
		expectedFragmentID++
	}

	if !sorted[len(sorted)-1].End {
		return nil, fmt.Errorf("psrp fragments do not end with an End fragment")
	}

	return payload.Bytes(), nil
}

func DecodeMessages(raw []byte) ([]Message, error) {
	fragments, err := DecodeFragments(raw)
	if err != nil {
		return nil, err
	}

	grouped := map[uint64][]Fragment{}
	objectIDs := make([]uint64, 0, len(fragments))
	for _, fragment := range fragments {
		if _, ok := grouped[fragment.ObjectID]; !ok {
			objectIDs = append(objectIDs, fragment.ObjectID)
		}
		grouped[fragment.ObjectID] = append(grouped[fragment.ObjectID], fragment)
	}
	sort.Slice(objectIDs, func(i, j int) bool { return objectIDs[i] < objectIDs[j] })

	messages := make([]Message, 0, len(objectIDs))
	for _, objectID := range objectIDs {
		payload, err := AssembleFragments(grouped[objectID])
		if err != nil {
			return nil, err
		}
		message, err := DecodeMessage(payload)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	return messages, nil
}

func BuildCreationXML(fragments []Fragment) ([]byte, error) {
	var payload bytes.Buffer
	for _, fragment := range fragments {
		payload.Write(EncodeFragment(fragment))
	}

	document := struct {
		XMLName xml.Name `xml:"creationXml"`
		XMLNS   string   `xml:"xmlns,attr"`
		Value   string   `xml:",chardata"`
	}{
		XMLNS: "http://schemas.microsoft.com/powershell",
		Value: base64.StdEncoding.EncodeToString(payload.Bytes()),
	}

	return xml.Marshal(document)
}

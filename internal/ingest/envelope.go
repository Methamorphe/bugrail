package ingest

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// EnvelopeItem is a parsed item from a Sentry envelope.
type EnvelopeItem struct {
	Type        string
	Payload     []byte
	Filename    string
	ContentType string
}

type envelopeHeader struct {
	EventID string `json:"event_id"`
}

type itemHeader struct {
	Type        string `json:"type"`
	Length      int    `json:"length"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
}

func parseEnvelope(body []byte) ([]EnvelopeItem, error) {
	line, pos, err := readLine(body, 0)
	if err != nil {
		return nil, fmt.Errorf("read envelope header: %w", err)
	}

	var header envelopeHeader
	if len(bytes.TrimSpace(line)) > 0 {
		if err := json.Unmarshal(line, &header); err != nil {
			return nil, fmt.Errorf("decode envelope header: %w", err)
		}
	}

	items := make([]EnvelopeItem, 0, 2)
	for pos < len(body) {
		if pos == len(body) {
			break
		}
		line, next, lineErr := readLine(body, pos)
		if lineErr != nil {
			return nil, fmt.Errorf("read item header: %w", lineErr)
		}
		pos = next
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var item itemHeader
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, fmt.Errorf("decode item header: %w", err)
		}
		if item.Type == "" {
			continue
		}

		payload, nextPos, payloadErr := readItemPayload(body, pos, item.Length)
		if payloadErr != nil {
			return nil, fmt.Errorf("read %s payload: %w", item.Type, payloadErr)
		}
		pos = nextPos
		items = append(items, EnvelopeItem{
			Type:        item.Type,
			Payload:     payload,
			Filename:    item.Filename,
			ContentType: item.ContentType,
		})
	}

	return items, nil
}

func readItemPayload(body []byte, pos, length int) ([]byte, int, error) {
	if length > 0 {
		if pos+length > len(body) {
			return nil, pos, fmt.Errorf("payload length %d exceeds body", length)
		}
		payload := body[pos : pos+length]
		pos += length
		if pos < len(body) && body[pos] == '\n' {
			pos++
		}
		return payload, pos, nil
	}

	line, next, err := readLine(body, pos)
	if err != nil {
		return nil, pos, err
	}
	return line, next, nil
}

func readLine(body []byte, pos int) ([]byte, int, error) {
	if pos >= len(body) {
		return nil, pos, fmt.Errorf("unexpected end of envelope")
	}
	idx := bytes.IndexByte(body[pos:], '\n')
	if idx == -1 {
		return body[pos:], len(body), nil
	}
	line := body[pos : pos+idx]
	return line, pos + idx + 1, nil
}

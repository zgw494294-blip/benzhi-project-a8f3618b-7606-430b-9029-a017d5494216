package store

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func frameDigest(frame eventFrame) string {
	frame.Checksum = ""
	raw, _ := json.Marshal(frame)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func (s *FileStore) appendFrame(key string, payload committedPayload) (eventFrame, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return eventFrame{}, err
	}
	frame := eventFrame{SchemaVersion: schemaVersion, Sequence: s.state.LastSequence + 1, PreviousDigest: s.state.LastDigest, Kind: "commit", CaseID: payload.Case.ID, IdempotencyKey: key, Payload: raw}
	frame.Checksum = frameDigest(frame)
	encoded, err := json.Marshal(frame)
	if err != nil {
		return eventFrame{}, err
	}
	if len(encoded) > 16<<20 {
		return eventFrame{}, fmt.Errorf("事件帧过大")
	}
	var buf bytes.Buffer
	if err = binary.Write(&buf, binary.BigEndian, uint32(len(encoded))); err != nil {
		return eventFrame{}, err
	}
	buf.Write(encoded)
	if _, err = s.log.Write(buf.Bytes()); err != nil {
		return eventFrame{}, err
	}
	if err = s.log.Sync(); err != nil {
		return eventFrame{}, err
	}
	return frame, nil
}
func readFrames(path string) ([]eventFrame, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	frames := []eventFrame{}
	for {
		var size uint32
		err = binary.Read(reader, binary.BigEndian, &size)
		if err == io.EOF {
			return frames, nil
		}
		if err != nil {
			return nil, fmt.Errorf("事件日志长度前缀损坏: %w", err)
		}
		if size == 0 || size > 16<<20 {
			return nil, fmt.Errorf("事件帧长度无效: %d", size)
		}
		raw := make([]byte, size)
		if _, err = io.ReadFull(reader, raw); err != nil {
			return nil, fmt.Errorf("事件日志截断: %w", err)
		}
		var frame eventFrame
		if err = json.Unmarshal(raw, &frame); err != nil {
			return nil, fmt.Errorf("事件帧 JSON 损坏: %w", err)
		}
		frames = append(frames, frame)
	}
}

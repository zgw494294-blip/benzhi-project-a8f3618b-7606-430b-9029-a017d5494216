package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type IntegrityReport struct {
	SchemaVersion int    `json:"schemaVersion"`
	EventCount    uint64 `json:"eventCount"`
	LastDigest    string `json:"lastDigest"`
	SnapshotValid bool   `json:"snapshotValid"`
}

func readProjection(path string) (*projection, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var value projection
	if err = json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("投影快照 JSON 损坏: %w", err)
	}
	if value.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf("投影 schemaVersion 不支持: %d", value.SchemaVersion)
	}
	if value.Checksum == "" || projectionDigest(value) != value.Checksum {
		return nil, fmt.Errorf("投影快照校验摘要不匹配")
	}
	return &value, nil
}
func VerifyFiles(dir string) (*IntegrityReport, error) {
	frames, err := readFrames(filepath.Join(dir, "events.rglog"))
	if err != nil {
		return nil, err
	}
	previous := ""
	for index, frame := range frames {
		if frame.Sequence != uint64(index+1) {
			return nil, fmt.Errorf("事件序号不连续")
		}
		if frame.PreviousDigest != previous || frameDigest(frame) != frame.Checksum {
			return nil, fmt.Errorf("事件摘要链无效")
		}
		previous = frame.Checksum
	}
	snapshot, err := readProjection(filepath.Join(dir, "projection.json"))
	if err != nil {
		return nil, err
	}
	report := &IntegrityReport{SchemaVersion: schemaVersion, EventCount: uint64(len(frames)), LastDigest: previous, SnapshotValid: snapshot != nil}
	if snapshot != nil {
		if snapshot.LastSequence > report.EventCount {
			return nil, fmt.Errorf("投影序号超前于事件日志")
		}
		if snapshot.LastSequence == report.EventCount && snapshot.LastDigest != previous {
			return nil, fmt.Errorf("投影摘要与事件日志不一致")
		}
	}
	return report, nil
}

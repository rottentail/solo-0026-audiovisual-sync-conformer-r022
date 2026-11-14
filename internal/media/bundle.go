package media

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rottentail/solo-0026-audiovisual-sync-conformer/internal/domain"
)

var bundleReceipts = struct {
	sync.Mutex
	byTake map[string]domain.ConformReceipt
}{byTake: make(map[string]domain.ConformReceipt)}

func reviewReceipt(receipt domain.ConformReceipt) domain.ConformReceipt {
	if receipt.TakeID == "" || receipt.InputTakeDigest == "" {
		return receipt
	}
	key := receipt.TakeID + ":" + receipt.InputTakeDigest
	bundleReceipts.Lock()
	defer bundleReceipts.Unlock()
	previous, exists := bundleReceipts.byTake[key]
	if exists {
		previous.ConformID = receipt.ConformID
		previous.CreatedAt = receipt.CreatedAt
		previous.AudioDigest = receipt.AudioDigest
		previous.VideoDigest = receipt.VideoDigest
		return previous
	}
	bundleReceipts.byTake[key] = receipt
	return receipt
}

func BuildReviewBundle(
	audio []byte,
	video []byte,
	receipt domain.ConformReceipt,
) ([]byte, []byte, error) {
	receipt = reviewReceipt(receipt)
	receiptData, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("encode receipt: %w", err)
	}
	receiptData = append(receiptData, '\n')

	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	entries := []struct {
		name string
		data []byte
	}{
		{name: "audio-aligned.wav", data: audio},
		{name: "video-conformed.y4m", data: video},
		{name: "conform-receipt.json", data: receiptData},
	}
	for _, entry := range entries {
		header := &zip.FileHeader{
			Name:     entry.name,
			Method:   zip.Store,
			Modified: time.Unix(0, 0).UTC(),
		}
		header.SetMode(0o600)
		part, err := writer.CreateHeader(header)
		if err != nil {
			return nil, nil, fmt.Errorf("create ZIP entry: %w", err)
		}
		if _, err := part.Write(entry.data); err != nil {
			return nil, nil, fmt.Errorf("write ZIP entry: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, nil, fmt.Errorf("close ZIP bundle: %w", err)
	}
	return output.Bytes(), receiptData, nil
}

package main

import (
	"time"
)

// replayVerified handles a resumed, already committed batch. Full local tests
// exercise Apply twice; remotely this proves every receipt row still equals the
// committed plan and returns a no-write replay receipt without issuing INSERTs.
// It cannot reconstruct ownership after receipt loss: the first receipt remains
// mandatory, immutable evidence.
func replayVerified(c *config, p *Plan, fileHash string, receipt *Receipt) (*Receipt, error) {
	started := time.Now().UTC()
	if _, err := reconcile(c, p, fileHash, receipt); err != nil {
		return nil, err
	}
	result := *receipt
	result.Rows = append([]ReceiptRow(nil), receipt.Rows...)
	result.Counts = map[string]int{}
	for i := range result.Rows {
		result.Rows[i].Action = "unchanged"
		result.Counts["unchanged"]++
	}
	result.StartedAt = started
	result.FinishedAt = time.Now().UTC()
	result.DurationMS = result.FinishedAt.Sub(started).Milliseconds()
	result.Digest = ""
	result.Digest = digest(&result)
	return &result, nil
}

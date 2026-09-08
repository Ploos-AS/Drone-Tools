package main

import (
	"testing"

	"github.com/Ploos-AS/Drone-Tools/internal/health"
	"github.com/Ploos-AS/Drone-Tools/internal/tlog"
)

func TestAccumulateTLOGLinkTracksLowBufferAndErrorIncreases(t *testing.T) {
	samples := []tlog.RadioSample{
		{SystemID: 1, ComponentID: 68, TxBufferPct: 80, RxErrors: 10},
		{SystemID: 1, ComponentID: 68, TxBufferPct: 15, RxErrors: 12},
		{SystemID: 1, ComponentID: 68, TxBufferPct: 10, RxErrors: 12},
		{SystemID: 2, ComponentID: 68, TxBufferPct: 90, RxErrors: 100},
		{SystemID: 2, ComponentID: 68, TxBufferPct: 90, RxErrors: 101},
	}
	var input health.LinkInput
	accumulateTLOGLink(&input, samples)
	if input.Samples != 5 || input.LowTxBufferSamples != 2 || input.RxErrorIncreaseEvents != 2 {
		t.Fatalf("unexpected link input: %+v", input)
	}
}

func TestHealthInputFromTLOGDetailedIncludesRadio(t *testing.T) {
	summary := tlog.DetailedSummary{
		Summary: tlog.Summary{
			Telemetry: tlog.Telemetry{
				GPS: []tlog.GPSSample{
					{Source: "GLOBAL_POSITION_INT", SystemID: 1, ComponentID: 1, TimestampUS: 1_000_000, Latitude: 58, Longitude: 7},
					{Source: "GLOBAL_POSITION_INT", SystemID: 1, ComponentID: 1, TimestampUS: 2_000_000, Latitude: 58.001, Longitude: 7.001},
				},
			},
		},
		Radio: []tlog.RadioSample{
			{SystemID: 1, ComponentID: 68, TxBufferPct: 50, RxErrors: 2},
			{SystemID: 1, ComponentID: 68, TxBufferPct: 5, RxErrors: 3},
		},
	}
	input := healthInputFromTLOGDetailed(summary)
	if input.Link.Samples != 2 || input.Link.LowTxBufferSamples != 1 || input.Link.RxErrorIncreaseEvents != 1 {
		t.Fatalf("unexpected link input: %+v", input.Link)
	}
}

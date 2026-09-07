package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"net/http"
	"testing"

	"github.com/Ploos-AS/Drone-Tools/internal/mapdata"
	"github.com/Ploos-AS/Drone-Tools/internal/tlog"
)

func TestTLOGTelemetryEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	fixture := buildTLOGWorkspaceFixture()
	rr := postFile(t, handler, "/api/v1/tlog/telemetry", "flight.tlog", fixture)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var summary tlog.Summary
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if summary.Format != "mavlink-tlog" || len(summary.Telemetry.GPS) != 2 || len(summary.Telemetry.Battery) != 1 {
		t.Fatalf("unexpected TLOG telemetry: %+v", summary)
	}
}

func TestTLOGWorkspaceEndpoints(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	fixture := buildTLOGWorkspaceFixture()

	mapRR := postFile(t, handler, "/api/v1/map", "flight.tlog", fixture)
	if mapRR.Code != http.StatusOK {
		t.Fatalf("map status = %d body=%s", mapRR.Code, mapRR.Body.String())
	}
	var doc mapdata.Document
	if err := json.NewDecoder(mapRR.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc.Format != "mavlink-tlog" || len(doc.Paths) != 1 || len(doc.Paths[0]) != 2 || doc.Bounds == nil {
		t.Fatalf("unexpected TLOG map: %+v", doc)
	}

	analysisRR := postFile(t, handler, "/api/v1/analyze", "flight.tlog", fixture)
	if analysisRR.Code != http.StatusOK {
		t.Fatalf("analysis status = %d body=%s", analysisRR.Code, analysisRR.Body.String())
	}
	var analysis flightAnalysis
	if err := json.NewDecoder(analysisRR.Body).Decode(&analysis); err != nil {
		t.Fatal(err)
	}
	if analysis.TrackPoints != 2 || analysis.DistanceMeters <= 0 || analysis.DurationSeconds == nil || analysis.BatteryVoltageV == nil || analysis.BatteryRemaining == nil {
		t.Fatalf("unexpected TLOG analysis: %+v", analysis)
	}
	if math.Abs(*analysis.BatteryVoltageV-15.2) > 1e-6 || math.Abs(*analysis.BatteryRemaining-0.75) > 1e-6 {
		t.Fatalf("unexpected TLOG battery metrics: %+v", analysis)
	}
	if math.Abs(analysis.ElevationGainMeters-2) > 1e-6 {
		t.Fatalf("unexpected TLOG elevation metrics: %+v", analysis)
	}
}

func buildTLOGWorkspaceFixture() string {
	var b bytes.Buffer
	writeTLOGRecord(&b, 1_000_000, mavlink1TestFrame(1, 1, 24, gpsRawPayload(1_000_000, 580000000, 70000000, 100000, 1000, 3, 12)))
	writeTLOGRecord(&b, 3_000_000, mavlink1TestFrame(1, 1, 24, gpsRawPayload(3_000_000, 580010000, 70020000, 102000, 1200, 3, 11)))
	writeTLOGRecord(&b, 3_100_000, mavlink1TestFrame(1, 1, 1, sysStatusPayload(15200, 650, 75)))
	return b.String()
}

func writeTLOGRecord(b *bytes.Buffer, timestamp uint64, frame []byte) {
	_ = binary.Write(b, binary.BigEndian, timestamp)
	b.Write(frame)
}

func mavlink1TestFrame(sysID, compID, msgID byte, payload []byte) []byte {
	frame := []byte{0xFE, byte(len(payload)), 1, sysID, compID, msgID}
	frame = append(frame, payload...)
	return append(frame, 0, 0)
}

func gpsRawPayload(timestamp uint64, lat, lon, alt int32, velocity uint16, fix, satellites byte) []byte {
	payload := make([]byte, 30)
	binary.LittleEndian.PutUint64(payload[0:8], timestamp)
	payload[8] = fix
	binary.LittleEndian.PutUint32(payload[9:13], uint32(lat))
	binary.LittleEndian.PutUint32(payload[13:17], uint32(lon))
	binary.LittleEndian.PutUint32(payload[17:21], uint32(alt))
	binary.LittleEndian.PutUint16(payload[25:27], velocity)
	payload[29] = satellites
	return payload
}

func sysStatusPayload(voltageMV uint16, currentCA int16, remaining byte) []byte {
	payload := make([]byte, 19)
	binary.LittleEndian.PutUint16(payload[14:16], voltageMV)
	binary.LittleEndian.PutUint16(payload[16:18], uint16(currentCA))
	payload[18] = remaining
	return payload
}

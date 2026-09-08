package tlog

import (
	"bytes"
	"fmt"
	"io"
	"sort"
)

const (
	msgHeartbeat        = 0
	heartbeatCRCExtra   = 50
	mavAutopilotInvalid = 8
	mavTypeGCS          = 6
	mavTypeOnboard      = 18
)

type EndpointRole struct {
	SystemID    uint8  `json:"system_id"`
	ComponentID uint8  `json:"component_id"`
	MAVType     uint8  `json:"mav_type"`
	Autopilot   uint8  `json:"autopilot"`
	Role        string `json:"role"`
}

type DetailedSummary struct {
	Summary
	EndpointRoles []EndpointRole `json:"endpoint_roles,omitempty"`
	Radio         []RadioSample  `json:"radio,omitempty"`
}

func InspectDetailed(r io.Reader) (DetailedSummary, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return DetailedSummary{}, err
	}
	summary, err := Inspect(bytes.NewReader(data))
	if err != nil {
		return DetailedSummary{}, err
	}
	roles, err := inspectEndpointRoles(data)
	if err != nil {
		return DetailedSummary{}, err
	}
	radio, err := inspectRadioStatus(data)
	if err != nil {
		return DetailedSummary{}, err
	}
	return DetailedSummary{Summary: summary, EndpointRoles: roles, Radio: radio}, nil
}

func inspectEndpointRoles(data []byte) ([]EndpointRole, error) {
	roles := map[uint16]EndpointRole{}
	for offset := 0; offset < len(data); {
		if len(data)-offset < 9 {
			return nil, ErrTruncated
		}
		frameStart := offset + 8
		frameLength, msgID, sysID, compID, _, version, err := frameInfo(data[frameStart:])
		if err != nil {
			next, ok := findNextRecord(data, offset+1)
			if !ok {
				return nil, err
			}
			offset = next
			continue
		}
		frame := data[frameStart : frameStart+frameLength]
		if msgID == msgHeartbeat {
			if !validFrameChecksum(frame, version, heartbeatCRCExtra) {
				return nil, fmt.Errorf("%w: message %d", ErrChecksum, msgID)
			}
			payload := framePayload(frame, version)
			if len(payload) >= 9 {
				mavType := payload[4]
				autopilot := payload[5]
				role := "component"
				switch {
				case autopilot != mavAutopilotInvalid:
					role = "flight-controller"
				case mavType == mavTypeGCS:
					role = "gcs"
				case mavType == mavTypeOnboard:
					role = "onboard-controller"
				}
				key := uint16(sysID)<<8 | uint16(compID)
				roles[key] = EndpointRole{SystemID: sysID, ComponentID: compID, MAVType: mavType, Autopilot: autopilot, Role: role}
			}
		}
		offset = frameStart + frameLength
	}
	out := make([]EndpointRole, 0, len(roles))
	for _, role := range roles {
		out = append(out, role)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SystemID == out[j].SystemID {
			return out[i].ComponentID < out[j].ComponentID
		}
		return out[i].SystemID < out[j].SystemID
	})
	return out, nil
}

func IsFlightController(roles []EndpointRole, systemID, componentID uint8) bool {
	for _, role := range roles {
		if role.SystemID == systemID && role.ComponentID == componentID {
			return role.Role == "flight-controller"
		}
	}
	return false
}

package main

import (
	"testing"

	"github.com/zboyco/jtt809/pkg/jtt809"
)

func TestParseVehicleRegistration(t *testing.T) {
	// Helper to encode GBK string
	encode := func(s string) []byte {
		b, _ := jtt809.EncodeGBK(s)
		return b
	}

	// Helper to create payload
	createPayload := func(version string, platform, producer, model, imei, termID, sim string) []byte {
		var (
			lenPlatform = 11
			lenProducer = 11
			lenModel    = 20
			lenIMEI     = 15
			lenTermID   = 7
			lenSIM      = 12
		)
		if version == "2019" {
			lenModel = 30
			lenIMEI = 30
			lenTermID = 30
			lenSIM = 13
		}

		buf := make([]byte, 0)

		pad := func(b []byte, length int) []byte {
			padded := make([]byte, length)
			copy(padded, b)
			return padded
		}

		buf = append(buf, pad(encode(platform), lenPlatform)...)
		buf = append(buf, pad(encode(producer), lenProducer)...)
		buf = append(buf, pad(encode(model), lenModel)...)
		buf = append(buf, pad(encode(imei), lenIMEI)...)
		buf = append(buf, pad(encode(termID), lenTermID)...)
		buf = append(buf, pad(encode(sim), lenSIM)...)
		return buf
	}

	t.Run("Version 2011", func(t *testing.T) {
		payload := createPayload("2011", "Plat1", "Prod1", "Model1", "IMEI1", "TID1", "SIM1")
		reg, err := parseVehicleRegistration(payload, "2011")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reg.PlatformID != "Plat1" {
			t.Errorf("expected Plat1, got %s", reg.PlatformID)
		}
		if reg.TerminalModelType != "Model1" {
			t.Errorf("expected Model1, got %s", reg.TerminalModelType)
		}
	})

	t.Run("Version 2019", func(t *testing.T) {
		payload := createPayload("2019", "Plat2", "Prod2", "Model2", "IMEI2", "TID2", "SIM2")
		reg, err := parseVehicleRegistration(payload, "2019")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reg.PlatformID != "Plat2" {
			t.Errorf("expected Plat2, got %s", reg.PlatformID)
		}
		if reg.TerminalModelType != "Model2" {
			t.Errorf("expected Model2, got %s", reg.TerminalModelType)
		}
	})

	t.Run("Version Mismatch (2011 payload with 2019 config)", func(t *testing.T) {
		payload := createPayload("2011", "Plat1", "Prod1", "Model1", "IMEI1", "TID1", "SIM1")
		_, err := parseVehicleRegistration(payload, "2019")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

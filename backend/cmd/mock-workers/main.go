package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"sthira/backend/internal/contracts"
)

type scenario string

const (
	scenarioDefault          scenario = "default"
	scenarioSilentZoom       scenario = "silent-zoom"
	scenarioDestinationChoice scenario = "destination-choice"
	scenarioArrivalConfirm  scenario = "arrival-confirm"
	scenarioClarify          scenario = "clarify"
	scenarioDataUnavailable  scenario = "data-unavailable"
	scenarioWorkerFailure    scenario = "worker-failure"
)

var validScenarios = map[scenario]bool{
	scenarioDefault:          true,
	scenarioSilentZoom:       true,
	scenarioDestinationChoice: true,
	scenarioArrivalConfirm:   true,
	scenarioClarify:          true,
	scenarioDataUnavailable:  true,
	scenarioWorkerFailure:    true,
}

var scenarioDescriptions = map[scenario]string{
	scenarioDefault:          "Default: FOCUS_FEATURE SZDEMO-1 + destination_options (backward compatible)",
	scenarioSilentZoom:       "Silent zoom: ZOOM IN action, no speech",
	scenarioDestinationChoice: "Destination choice: SHOW_CHOICES FACDEMO-1 + destination_options audio",
	scenarioArrivalConfirm:   "Arrival confirm: OPEN_PANEL ARRIVAL_CONFIRMATION only",
	scenarioClarify:          "Clarify: CLARIFY with SZDEMO-1 and FACDEMO-1 candidates",
	scenarioDataUnavailable:  "Data unavailable: DATA_UNAVAILABLE status, no actions",
	scenarioWorkerFailure:    "Worker failure: Middle worker returns 503 error",
}

// generateWAVBytes creates a minimal valid PCM WAV buffer for mock TTS audio.
func generateWAVBytes(sampleCount int) []byte {
	subChunk2Size := sampleCount * 2
	chunkSize := 36 + subChunk2Size
	data := make([]byte, 44+subChunk2Size)
	copy(data[0:4], "RIFF")
	data[4] = byte(chunkSize)
	data[5] = byte(chunkSize >> 8)
	data[6] = byte(chunkSize >> 16)
	data[7] = byte(chunkSize >> 24)
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	data[16] = 16
	data[20] = 1 // PCM
	data[22] = 1 // Mono
	data[24] = 0x80
	data[25] = 0x3E // 16000 Hz
	data[28] = 0x00
	data[29] = 0x7D // Byte rate = 32000
	data[32] = 2    // Block align = 2
	data[34] = 16   // 16 bits
	copy(data[36:40], "data")
	data[40] = byte(subChunk2Size)
	data[41] = byte(subChunk2Size >> 8)
	data[42] = byte(subChunk2Size >> 16)
	data[43] = byte(subChunk2Size >> 24)
	return data
}

func main() {
	port := flag.Int("port", 0, "port to listen on (0 for ephemeral)")
	scenarioFlag := flag.String("scenario", string(scenarioDefault), "Scenario: default|silent-zoom|destination-choice|arrival-confirm|clarify|data-unavailable|worker-failure")
	flag.Parse()

	sc := scenario(*scenarioFlag)
	if !validScenarios[sc] {
		log.Fatalf("invalid scenario %q, valid: silent-zoom|destination-choice|arrival-confirm|clarify|data-unavailable|worker-failure", *scenarioFlag)
	}
	fmt.Printf("SCENARIO=%s\n", sc)
	fmt.Printf("SCENARIO_DESC=%s\n", scenarioDescriptions[sc])

	wavBytes := generateWAVBytes(400)
	wavB64 := base64.StdEncoding.EncodeToString(wavBytes)
	wavHash := sha256.Sum256(wavBytes)
	wavChecksum := hex.EncodeToString(wavHash[:])

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(contracts.WorkerHealth{
			Ready:              true,
			Warm:               true,
			SupportedLanguages: []string{"en-IN", "hi-IN", "ml-IN"},
		})
	})

	mux.HandleFunc("/transcribe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req contracts.ASRWorkerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		text := "Meppadi"
		lang := req.Language
		if lang == "" {
			lang = "en-IN"
		}
		if lang == "ml-IN" {
			text = "മേപ്പാടി സുരക്ഷിത കേന്ദ്രം"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(contracts.ASRWorkerResponse{
			RequestID:      req.RequestID,
			Language:       lang,
			Text:           text,
			State:          contracts.TranscriptionOK,
			ModelRevision:  "indic-conformer-600m-v1",
			ArtifactDigest: "sha256-conformer-600m-demo",
		})
	})

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if sc == scenarioWorkerFailure {
			http.Error(w, `{"error":"model unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		var req contracts.MiddleWorkerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		lang := req.Transcript.Language
		if lang == "" {
			lang = "en-IN"
		}
		dataVer := req.ScopedContext.DataVersion
		if dataVer == "" {
			dataVer = "PKGDEMO-1:1"
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(buildMiddleResponse(sc, req.RequestID, dataVer, lang))
	})

	mux.HandleFunc("/synthesize", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req contracts.TTSWorkerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.SpeechKey == "" {
			http.Error(w, "speech_key required", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(contracts.TTSWorkerResponse{
			RequestID:      req.RequestID,
			SpeechKey:      req.SpeechKey,
			Language:       req.Language,
			State:          contracts.TTSOK,
			AudioB64:       wavB64,
			ContentType:    "audio/wav",
			ChecksumSHA256: wavChecksum,
			ModelRevision:  "indic-parler-tts-v1",
			VoiceRevision:  "voice-default-v1",
			Settings: contracts.TTSSynthesisSettings{
				SampleRate: 16000,
				BitDepth:   16,
				Channels:   1,
			},
		})
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer listener.Close()

	actualPort := listener.Addr().(*net.TCPAddr).Port
	fmt.Printf("WORKER_URL=http://127.0.0.1:%d\n", actualPort)

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("worker serve error: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	_ = srv.Close()
}

func buildMiddleResponse(sc scenario, requestID, dataVer, lang string) contracts.MiddleWorkerResponse {
	switch sc {
	case scenarioSilentZoom:
		intent := contracts.IntentZoom
		return contracts.MiddleWorkerResponse{
			RequestID:   requestID,
			DataVersion: dataVer,
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:        requestID,
				DataVersion:      dataVer,
				Status:           contracts.StatusOK,
				Intent:           &intent,
				Language:         lang,
				Actions:          []contracts.Action{{Type: contracts.ActionZoom, Direction: "IN", Steps: 1}},
				SpeechKey:        nil,
				ClarificationIDs: []string{},
				EvidenceIDs:      []string{},
			},
			ModelRevision: "sarvamai/sarvam-30b-fp8",
		}

	case scenarioDestinationChoice:
		intent := contracts.IntentListDestinations
		sk := "destination_options"
		return contracts.MiddleWorkerResponse{
			RequestID:   requestID,
			DataVersion: dataVer,
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:        requestID,
				DataVersion:      dataVer,
				Status:           contracts.StatusOK,
				Intent:           &intent,
				Language:         lang,
				Actions:          []contracts.Action{{Type: contracts.ActionShowChoices, TargetIDs: []string{"FACDEMO-1"}}},
				SpeechKey:        &sk,
				ClarificationIDs: []string{},
				EvidenceIDs:      []string{"FACDEMO-1"},
			},
			ModelRevision: "sarvamai/sarvam-30b-fp8",
		}

	case scenarioArrivalConfirm:
		intent := contracts.IntentOpenConfirmation
		panel := contracts.PanelArrivalConfirm
		return contracts.MiddleWorkerResponse{
			RequestID:   requestID,
			DataVersion: dataVer,
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:        requestID,
				DataVersion:      dataVer,
				Status:           contracts.StatusOK,
				Intent:           &intent,
				Language:         lang,
				Actions:          []contracts.Action{{Type: contracts.ActionOpenPanel, Panel: panel}},
				SpeechKey:        nil,
				ClarificationIDs: []string{},
				EvidenceIDs:      []string{},
			},
			ModelRevision: "sarvamai/sarvam-30b-fp8",
		}

	case scenarioClarify:
		sk := "clarify_place"
		return contracts.MiddleWorkerResponse{
			RequestID:   requestID,
			DataVersion: dataVer,
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:        requestID,
				DataVersion:      dataVer,
				Status:           contracts.StatusClarify,
				Intent:           nil,
				Language:         lang,
				Actions:          []contracts.Action{},
				SpeechKey:        &sk,
				ClarificationIDs: []string{"SZDEMO-1", "FACDEMO-1"},
				EvidenceIDs:      []string{},
			},
			ModelRevision: "sarvamai/sarvam-30b-fp8",
		}

	case scenarioDataUnavailable:
		return contracts.MiddleWorkerResponse{
			RequestID:   requestID,
			DataVersion: dataVer,
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:        requestID,
				DataVersion:      dataVer,
				Status:           contracts.StatusDataUnavailable,
				Intent:           nil,
				Language:         lang,
				Actions:          []contracts.Action{},
				SpeechKey:        nil,
				ClarificationIDs: []string{},
				EvidenceIDs:      []string{},
			},
			ModelRevision: "sarvamai/sarvam-30b-fp8",
		}

	default:
		intent := contracts.IntentFocusPlace
		sk := "destination_options"
		return contracts.MiddleWorkerResponse{
			RequestID:   requestID,
			DataVersion: dataVer,
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:        requestID,
				DataVersion:      dataVer,
				Status:           contracts.StatusOK,
				Intent:           &intent,
				Language:         lang,
				Actions:          []contracts.Action{{Type: contracts.ActionFocusFeature, TargetID: "SZDEMO-1"}},
				SpeechKey:        &sk,
				ClarificationIDs: []string{},
				EvidenceIDs:      []string{"SZDEMO-1"},
			},
			ModelRevision: "sarvamai/sarvam-30b-fp8",
		}
	}
}

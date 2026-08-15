package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	apiKey := strings.TrimSpace(os.Getenv("DEEPGRAM_API_KEY"))
	token := strings.TrimSpace(os.Getenv("TURN_TOKEN"))
	if apiKey == "" || token == "" {
		log.Fatal("DEEPGRAM_API_KEY and TURN_TOKEN are required")
	}
	sttModel := envOr("DEEPGRAM_STT_MODEL", "nova-3")
	ttsModel := envOr("DEEPGRAM_TTS_MODEL", "aura-2-aries-en")
	turnURL := strings.TrimRight(envOr("TURN_URL", "http://127.0.0.1:8760"), "/")
	card := envOr("ALSA_CARD", "Headset")
	startRMS := envFloat("HEADSET_RMS_START", 800)
	stopRMS := envFloat("HEADSET_RMS_STOP", 400)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("headset sidecar starting card=%s turn=%s", card, turnURL)
	if err := listenLoop(ctx, card, apiKey, sttModel, ttsModel, turnURL, token, startRMS, stopRMS); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}

func listenLoop(ctx context.Context, card, apiKey, sttModel, ttsModel, turnURL, token string, startRMS, stopRMS float64) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		pcm, err := captureUtterance(ctx, card, startRMS, stopRMS)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("capture: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if len(pcm) == 0 {
			continue
		}
		wav := wrapWAV(pcm, sampleRate, 1)
		transcript, err := transcribe(ctx, apiKey, sttModel, wav)
		if err != nil {
			log.Printf("stt: %v", err)
			continue
		}
		if transcript == "" {
			continue
		}
		reply, err := submitTurn(ctx, turnURL, token, transcript)
		if err != nil {
			log.Printf("turn: %v", err)
			continue
		}
		if strings.TrimSpace(reply) == "" {
			continue
		}
		audio, err := speak(ctx, apiKey, ttsModel, reply)
		if err != nil {
			log.Printf("tts: %v", err)
			continue
		}
		if err := playWAV(ctx, card, audio); err != nil {
			log.Printf("play: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func captureUtterance(ctx context.Context, card string, startRMS, stopRMS float64) ([]byte, error) {
	device := fmt.Sprintf("plughw:CARD=%s,DEV=0", card)
	cmd := exec.CommandContext(ctx, "arecord",
		"-D", device,
		"-f", "S16_LE",
		"-c", "1",
		"-r", strconv.Itoa(sampleRate),
		"-t", "raw",
		"-q",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("arecord: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	v := newVAD(startRMS, stopRMS)
	buf := make([]byte, frameSamples*2)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(stdout, buf); err != nil {
			return nil, err
		}
		if out := v.push(buf); out != nil {
			return out, nil
		}
	}
}

func wrapWAV(pcm []byte, rate, channels int) []byte {
	byteRate := rate * channels * 2
	blockAlign := channels * 2
	dataLen := len(pcm)
	buf := bytes.NewBuffer(make([]byte, 0, 44+dataLen))
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(36+dataLen))
	buf.WriteString("WAVEfmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(buf, binary.LittleEndian, uint32(rate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(dataLen))
	buf.Write(pcm)
	return buf.Bytes()
}

func transcribe(ctx context.Context, apiKey, model string, wav []byte) (string, error) {
	u := "https://api.deepgram.com/v1/listen?model=" + model + "&smart_format=true&punctuate=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(wav))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Token "+apiKey)
	req.Header.Set("Content-Type", "audio/wav")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("listen HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Results struct {
			Channels []struct {
				Alternatives []struct {
					Transcript string `json:"transcript"`
				} `json:"alternatives"`
			} `json:"channels"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Results.Channels) == 0 || len(parsed.Results.Channels[0].Alternatives) == 0 {
		return "", nil
	}
	return strings.TrimSpace(parsed.Results.Channels[0].Alternatives[0].Transcript), nil
}

func speak(ctx context.Context, apiKey, model, text string) ([]byte, error) {
	runes := []rune(text)
	if len(runes) > 2000 {
		text = string(runes[:2000])
	}
	payload, _ := json.Marshal(map[string]string{"text": text})
	u := "https://api.deepgram.com/v1/speak?model=" + model + "&encoding=linear16&sample_rate=48000&container=wav"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("speak HTTP %d", resp.StatusCode)
	}
	return body, nil
}

func submitTurn(ctx context.Context, turnURL, token, text string) (string, error) {
	payload, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnURL+"/v1/turn", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("turn HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	return strings.TrimSpace(parsed.Text), nil
}

func playWAV(ctx context.Context, card string, wav []byte) error {
	device := fmt.Sprintf("plughw:CARD=%s,DEV=0", card)
	cmd := exec.CommandContext(ctx, "aplay", "-D", device, "-q")
	cmd.Stdin = bytes.NewReader(wav)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("aplay: %w %s", err, bytes.TrimSpace(out))
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

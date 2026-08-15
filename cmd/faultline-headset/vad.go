package main

import (
	"encoding/binary"
	"math"
)

const (
	sampleRate      = 16000
	frameSamples    = 320 // 20 ms
	prerollFrames   = 15  // 300 ms
	minSpeechFrames = 20  // 400 ms
	silenceFrames   = 50  // 1 s
	maxSpeechFrames = 750 // 15 s
)

func rmsInt16(pcm []byte) float64 {
	if len(pcm) < 2 {
		return 0
	}
	n := len(pcm) / 2
	var sum float64
	for i := 0; i < n; i++ {
		s := int16(binary.LittleEndian.Uint16(pcm[i*2:]))
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(n))
}

type vadState struct {
	startRMS float64
	stopRMS  float64

	speaking bool
	voiced   int
	silent   int
	preroll  [][]byte
	utter    []byte
}

func newVAD(startRMS, stopRMS float64) *vadState {
	if startRMS <= 0 {
		startRMS = 800
	}
	if stopRMS <= 0 {
		stopRMS = 400
	}
	return &vadState{startRMS: startRMS, stopRMS: stopRMS}
}

// push returns a complete utterance (including preroll) or nil.
func (v *vadState) push(frame []byte) []byte {
	cp := append([]byte(nil), frame...)
	energy := rmsInt16(cp)
	if !v.speaking {
		v.preroll = append(v.preroll, cp)
		if len(v.preroll) > prerollFrames {
			v.preroll = v.preroll[len(v.preroll)-prerollFrames:]
		}
		if energy >= v.startRMS {
			v.voiced++
		} else {
			v.voiced = 0
		}
		if v.voiced >= 3 {
			v.speaking = true
			v.silent = 0
			v.utter = nil
			for _, p := range v.preroll {
				v.utter = append(v.utter, p...)
			}
			v.preroll = nil
		}
		return nil
	}
	v.utter = append(v.utter, cp...)
	frames := len(v.utter) / (frameSamples * 2)
	if energy < v.stopRMS {
		v.silent++
	} else {
		v.silent = 0
	}
	if frames >= maxSpeechFrames || (v.silent >= silenceFrames && frames >= minSpeechFrames) {
		out := v.utter
		v.speaking = false
		v.voiced = 0
		v.silent = 0
		v.utter = nil
		v.preroll = nil
		return out
	}
	return nil
}

#!/usr/bin/env python3
"""Read 16 kHz S16_LE mono from stdin in 80 ms chunks; print WAKE or '.' per chunk."""
import logging
import os
import sys

import numpy as np
from openwakeword.model import Model
from openwakeword.utils import download_models

logging.basicConfig(stream=sys.stderr, level=logging.WARNING)

MODEL = os.environ.get("WAKE_MODEL", "alexa")
THRESH = float(os.environ.get("WAKE_THRESHOLD", "0.5"))
SAMPLES = 1280

download_models([MODEL])
oww = Model(wakeword_models=[MODEL], inference_framework="onnx")
buf = sys.stdin.buffer
while True:
    data = buf.read(SAMPLES * 2)
    if len(data) < SAMPLES * 2:
        break
    audio = np.frombuffer(data, dtype=np.int16)
    scores = oww.predict(audio)
    hit = any(float(v) >= THRESH for v in scores.values())
    sys.stdout.write("WAKE\n" if hit else ".\n")
    sys.stdout.flush()

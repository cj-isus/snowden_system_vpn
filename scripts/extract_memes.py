#!/usr/bin/env python3
"""
Извлекает мемы Пепе из скриншота Snowden_system VPN.
Требует: pip install pillow
"""
from PIL import Image
import os

# Путь к исходному скриншоту
SRC = r"D:\User\Downloads\ChatGPT Image 9 июл. 2026 г., 11_02_02.png"
# Куда сохранять
OUT = r"D:\ОБХОДЫ\Snowden_system\memes"
os.makedirs(OUT, exist_ok=True)

img = Image.open(SRC)
w, h = img.size
print(f"[i] Исходник: {w}x{h}")

# (x1, y1, x2, y2) — координаты каждого мема
MEMES = {
    "01_pepe_hacker_fuck_dpi": (0, 45, 300, 345),
    "02_pepe_ushanka_star": (1320, 55, 1536, 280),
    "03_pepe_rkn_crying": (340, 610, 560, 800),
    "04_pepe_top_secret": (880, 470, 1110, 720),
    "05_pepe_laptop_logs": (1380, 610, 1536, 790),
    "06_pepe_shh_data": (0, 720, 280, 980),
    "07_pepe_taskbar": (1260, 960, 1350, 1024),
}

saved = []
for name, (x1, y1, x2, y2) in MEMES.items():
    x1, y1 = max(0, x1), max(0, y1)
    x2, y2 = min(w, x2), min(h, y2)
    crop = img.crop((x1, y1, x2, y2))
    if crop.mode in ("RGBA", "P"):
        crop = crop.convert("RGB")
    path = os.path.join(OUT, f"{name}.jpg")
    crop.save(path, "JPEG", quality=85, optimize=True)
    kb = os.path.getsize(path) / 1024
    saved.append(path)
    print(f"[+] {name}.jpg — {crop.size[0]}x{crop.size[1]} — {kb:.1f} KB")

print(f"\n[✓] Готово! {len(saved)} мемов в: {OUT}")

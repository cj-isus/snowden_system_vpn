from PIL import Image
import os

SRC = r"D:\User\Downloads\ChatGPT Image 9 июл. 2026 г., 11_02_02.png"
OUT = r"D:\ОБХОДЫ\Snowden_system\memes"
os.makedirs(OUT, exist_ok=True)

img = Image.open(SRC)
w, h = img.size

# Уточненные координаты (убраны куски интерфейса)
MEMES = {
    "01_pepe_hacker_fuck_dpi": (50, 50, 300, 340),
    "02_pepe_ushanka_star": (1330, 60, 1530, 270),
    "03_pepe_rkn_crying": (350, 625, 550, 790),
    "04_pepe_top_secret": (920, 480, 1100, 700),
    "05_pepe_laptop_logs": (1400, 630, 1530, 780),
    "06_pepe_shh_data": (30, 730, 260, 960),
    "07_pepe_taskbar": (1260, 970, 1340, 1020),
}

for name, (x1, y1, x2, y2) in MEMES.items():
    x1, y1 = max(0, x1), max(0, y1)
    x2, y2 = min(w, x2), min(h, y2)
    crop = img.crop((x1, y1, x2, y2))
    if crop.mode in ("RGBA", "P"):
        crop = crop.convert("RGB")
    path = os.path.join(OUT, f"{name}.jpg")
    crop.save(path, "JPEG", quality=90, optimize=True)
    kb = os.path.getsize(path) / 1024
    print(f"[OK] {name}.jpg  {crop.size[0]}x{crop.size[1]}  {kb:.1f} KB")

print(f"[DONE] Saved to: {OUT}")

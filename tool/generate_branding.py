#!/usr/bin/env python3
"""Generator aset branding Maritim Node.

Merasterisasi logo mesh milik dashboard frontend (frontend/src/app/icon.svg)
menjadi PNG untuk flutter_launcher_icons dan flutter_native_splash, sehingga
identitas visual mobile 100% konsisten dengan frontend tanpa menyalin file
biner antar-repo. Jalankan ulang bila logo frontend berubah:

    python3 tool/generate_branding.py

Output (assets/branding/):
  app_icon.png             1024x1024  ikon penuh (kotak navy membulat)
  app_icon_foreground.png  1024x1024  foreground adaptive icon (transparan)
  splash_logo.png           768x768   logo splash pra-Android 12 / iOS
  splash_logo_android12.png 1152x1152 varian Android 12 (aman masker lingkaran)
"""

from __future__ import annotations

import math
import os

from PIL import Image, ImageDraw

# Palet identik dengan frontend/src/app/icon.svg + globals.css.
NAVY = (15, 23, 42, 255)        # #0f172a — latar logo / slate-900
WAVE = (30, 58, 95, 255)        # #1e3a5f — ombak halus
LINK = (56, 189, 248, 255)      # #38bdf8 — garis link mesh / sky-400
SHIP = (16, 185, 129, 255)      # #10b981 — node kapal / emerald-500
EDGE = (34, 211, 238, 255)      # #22d3ee — node Syahbandar / cyan-400
TRANSPARENT = (0, 0, 0, 0)

OUT_DIR = os.path.join(os.path.dirname(__file__), "..", "assets", "branding")


def _bezier_points(p0, c1, c2, p1, steps=64):
    pts = []
    for i in range(steps + 1):
        t = i / steps
        mt = 1 - t
        x = mt**3 * p0[0] + 3 * mt**2 * t * c1[0] + 3 * mt * t**2 * c2[0] + t**3 * p1[0]
        y = mt**3 * p0[1] + 3 * mt**2 * t * c1[1] + 3 * mt * t**2 * c2[1] + t**3 * p1[1]
        pts.append((x, y))
    return pts


def _dashed_line(draw, p1, p2, dash, gap, width, fill):
    dx, dy = p2[0] - p1[0], p2[1] - p1[1]
    length = math.hypot(dx, dy)
    ux, uy = dx / length, dy / length
    pos = 0.0
    while pos < length:
        end = min(pos + dash, length)
        a = (p1[0] + ux * pos, p1[1] + uy * pos)
        b = (p1[0] + ux * end, p1[1] + uy * end)
        draw.line([a, b], fill=fill, width=width)
        pos = end + gap


def draw_logo(size_px: int, *, with_background: bool) -> Image.Image:
    """Menggambar logo pada kanvas persegi; koordinat sumber 64x64 (viewBox SVG)."""
    img = Image.new("RGBA", (size_px, size_px), TRANSPARENT)
    d = ImageDraw.Draw(img)
    s = size_px / 64.0  # skala dari ruang koordinat SVG

    if with_background:
        d.rounded_rectangle([0, 0, size_px - 1, size_px - 1], radius=14 * s, fill=NAVY)

    # Ombak: path M8 44 c6-6 10-2 16-6
    wave = _bezier_points((8 * s, 44 * s), (14 * s, 38 * s), (18 * s, 42 * s), (24 * s, 38 * s))
    d.line(wave, fill=WAVE, width=max(1, round(2 * s)), joint="curve")

    # Link mesh putus-putus (dasharray 4 3, lebar 2.5).
    lw = max(1, round(2.5 * s))
    _dashed_line(d, (18 * s, 46 * s), (32 * s, 20 * s), 4 * s, 3 * s, lw, LINK)
    _dashed_line(d, (32 * s, 20 * s), (48 * s, 42 * s), 4 * s, 3 * s, lw, LINK)

    # Node: dua kapal (emerald) + satu Syahbandar (cyan), r=6.
    for cx, cy, color in ((18, 46, SHIP), (32, 20, SHIP), (48, 42, EDGE)):
        r = 6 * s
        d.ellipse([cx * s - r, cy * s - r, cx * s + r, cy * s + r], fill=color)

    # Tanda plus pada node Syahbandar: M48 38 v8  m-3 -5 h6 (lebar 1.8).
    pw = max(1, round(1.8 * s))
    d.line([(48 * s, 38 * s), (48 * s, 46 * s)], fill=NAVY, width=pw)
    d.line([(45 * s, 41 * s), (51 * s, 41 * s)], fill=NAVY, width=pw)

    return img


def _centered(canvas_px: int, content: Image.Image) -> Image.Image:
    canvas = Image.new("RGBA", (canvas_px, canvas_px), TRANSPARENT)
    off = (canvas_px - content.width) // 2
    canvas.paste(content, (off, off), content)
    return canvas


def main() -> None:
    os.makedirs(OUT_DIR, exist_ok=True)
    # Supersampling 2x lalu downscale LANCZOS agar tepi anti-alias halus.
    def save(name: str, img: Image.Image, final: int) -> None:
        img.resize((final, final), Image.LANCZOS).save(os.path.join(OUT_DIR, name))
        print(f"  {name} ({final}x{final})")

    print("Menghasilkan aset branding:")
    save("app_icon.png", draw_logo(2048, with_background=True), 1024)

    # Adaptive foreground: konten harus muat di zona aman ±66% tengah kanvas.
    fg = draw_logo(1228, with_background=False)  # 2048 * 0.60
    save("app_icon_foreground.png", _centered(2048, fg), 1024)

    # Splash pra-Android 12 / iOS: ikon membulat 512 pada kanvas 768.
    icon_1024 = draw_logo(1024, with_background=True)
    save("splash_logo.png", _centered(1536, icon_1024), 768)

    # Android 12 memasker splash ke lingkaran 768/1152: sisi ikon harus
    # <= 768/sqrt(2) ≈ 543 px agar sudut kotak tidak terpotong.
    save("splash_logo_android12.png", _centered(2304, icon_1024), 1152)
    print("Selesai.")


if __name__ == "__main__":
    main()

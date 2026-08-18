from PIL import Image
import os
import struct
import io

input_path = r"D:\User\Downloads\ChatGPT Image 9 июл. 2026 г., 11_03_05.png"
output_dir = r"D:\ОБХОДЫ\Snowden_system\logo_assets"
os.makedirs(output_dir, exist_ok=True)

img = Image.open(input_path)

# Ensure square
if img.width != img.height:
    size = min(img.width, img.height)
    left = (img.width - size) // 2
    top = (img.height - size) // 2
    img = img.crop((left, top, left + size, top + size))

if img.mode != 'RGBA':
    img = img.convert('RGBA')

# Save PNG sizes
png_sizes = [16, 32, 64, 128, 256, 512]
for s in png_sizes:
    resized = img.resize((s, s), Image.LANCZOS)
    resized.save(os.path.join(output_dir, f"snowden_system_{s}x{s}.png"), 'PNG', optimize=True)

# Create ICO with multiple sizes using manual PNG embedding
# Windows Vista+ supports PNG data inside ICO files
ico_sizes = [16, 24, 32, 48, 64, 96, 128, 256]

# Generate PNG data for each size
png_data_list = []
for s in ico_sizes:
    resized = img.resize((s, s), Image.LANCZOS)
    buf = io.BytesIO()
    resized.save(buf, format='PNG', optimize=True)
    png_data_list.append(buf.getvalue())

ico_path = os.path.join(output_dir, "snowden_system_icon.ico")

with open(ico_path, 'wb') as f:
    # ICO header
    f.write(struct.pack('<HHH', 0, 1, len(ico_sizes)))  # Reserved, Type (icon), Count
    
    # Calculate directory offset and data offset
    dir_size = 6 + 16 * len(ico_sizes)
    data_offset = dir_size
    
    entries = []
    for i, s in enumerate(ico_sizes):
        data = png_data_list[i]
        size = len(data)
        # Directory entry: width(1), height(1), colors(1), reserved(1), planes(2), bpp(2), size(4), offset(4)
        # For 256x256, width and height are stored as 0
        w = 0 if s == 256 else s
        h = 0 if s == 256 else s
        entries.append((w, h, 0, 0, 1, 32, size, data_offset))
        data_offset += size
    
    for entry in entries:
        f.write(struct.pack('<BBBBHHII', *entry))
    
    for data in png_data_list:
        f.write(data)

# Verify the ICO
with open(ico_path, 'rb') as f:
    data = f.read()
    count = struct.unpack('<H', data[4:6])[0]
    print(f"ICO contains {count} images")
    for i in range(count):
        offset = 6 + i * 16
        width = data[offset] if data[offset] != 0 else 256
        height = data[offset + 1] if data[offset + 1] != 0 else 256
        size = struct.unpack('<I', data[offset + 8:offset + 12])[0]
        print(f"  Image {i+1}: {width}x{height}, {size} bytes")
    print(f"Total ICO size: {len(data):,} bytes")

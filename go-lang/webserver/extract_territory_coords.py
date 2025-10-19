#!/usr/bin/env python3
"""
Extract territory coordinates from the Axis & Allies map image using OCR.
This script detects text labels on the map and records their pixel positions.
"""

# /// script
# dependencies = [
#   "pillow",
#   "pytesseract",
#   "opencv-python",
#   "numpy",
# ]
# ///

import json
import cv2
import numpy as np
from PIL import Image
import pytesseract
from pathlib import Path

def extract_territory_coordinates(image_path):
    """
    Analyze the map image and extract territory name coordinates.

    Returns:
        dict: Territory names mapped to their pixel coordinates
    """
    print(f"Loading image: {image_path}")

    # Load image
    img = cv2.imread(str(image_path))
    if img is None:
        raise ValueError(f"Could not load image: {image_path}")

    height, width = img.shape[:2]
    print(f"Image dimensions: {width}x{height}")

    # Convert to RGB for PIL
    img_rgb = cv2.cvtColor(img, cv2.COLOR_BGR2RGB)
    pil_img = Image.fromarray(img_rgb)

    # Use Tesseract to detect text and get bounding boxes
    print("Running OCR to detect territory labels...")

    # Get detailed OCR data
    ocr_data = pytesseract.image_to_data(pil_img, output_type=pytesseract.Output.DICT)

    territories = {}
    current_text_block = []
    current_box = None

    # Process OCR results
    n_boxes = len(ocr_data['text'])
    for i in range(n_boxes):
        text = ocr_data['text'][i].strip()
        conf = int(ocr_data['conf'][i])

        # Skip empty text or low confidence
        if not text or conf < 30:
            # If we have accumulated text, save it
            if current_text_block and current_box:
                territory_name = ' '.join(current_text_block)
                if len(territory_name) > 2:  # Ignore single letters
                    x, y, w, h = current_box
                    center_x = x + w // 2
                    center_y = y + h // 2

                    # Calculate percentage coordinates
                    x_pct = (center_x / width) * 100
                    y_pct = (center_y / height) * 100

                    territories[territory_name] = {
                        'x': round(x_pct, 2),
                        'y': round(y_pct, 2),
                        'pixel_x': center_x,
                        'pixel_y': center_y,
                        'confidence': conf
                    }

                current_text_block = []
                current_box = None
            continue

        x = ocr_data['left'][i]
        y = ocr_data['top'][i]
        w = ocr_data['width'][i]
        h = ocr_data['height'][i]

        # Start or extend text block
        if not current_text_block:
            current_text_block = [text]
            current_box = [x, y, w, h]
        else:
            # Check if this text is close to the previous (part of same label)
            prev_x, prev_y, prev_w, prev_h = current_box
            if abs(y - prev_y) < 20 and abs(x - (prev_x + prev_w)) < 50:
                # Extend the text block
                current_text_block.append(text)
                # Extend bounding box
                new_x = min(x, prev_x)
                new_y = min(y, prev_y)
                new_right = max(x + w, prev_x + prev_w)
                new_bottom = max(y + h, prev_y + prev_h)
                current_box = [new_x, new_y, new_right - new_x, new_bottom - new_y]
            else:
                # Save previous block
                if current_text_block and current_box:
                    territory_name = ' '.join(current_text_block)
                    if len(territory_name) > 2:
                        bx, by, bw, bh = current_box
                        center_x = bx + bw // 2
                        center_y = by + bh // 2

                        x_pct = (center_x / width) * 100
                        y_pct = (center_y / height) * 100

                        territories[territory_name] = {
                            'x': round(x_pct, 2),
                            'y': round(y_pct, 2),
                            'pixel_x': center_x,
                            'pixel_y': center_y,
                            'confidence': conf
                        }

                # Start new block
                current_text_block = [text]
                current_box = [x, y, w, h]

    return territories, width, height

def main():
    # Path to the map image
    map_path = Path("/home/ccd/bg/go-lang/webserver/static/images/map.jpg")

    if not map_path.exists():
        print(f"Error: Map image not found at {map_path}")
        return

    # Extract coordinates
    territories, img_width, img_height = extract_territory_coordinates(map_path)

    print(f"\nFound {len(territories)} territory labels:")
    print("-" * 80)

    # Sort by name for easier reading
    for name in sorted(territories.keys()):
        coords = territories[name]
        print(f"{name:30s} -> ({coords['pixel_x']:4d}, {coords['pixel_y']:4d}) = ({coords['x']:6.2f}%, {coords['y']:6.2f}%) conf={coords['confidence']}")

    # Save to JSON file
    output_path = Path("/home/ccd/bg/go-lang/webserver/detected_territories.json")
    with open(output_path, 'w') as f:
        json.dump({
            'image_dimensions': {'width': img_width, 'height': img_height},
            'territories': territories
        }, f, indent=2)

    print(f"\n✓ Saved detected territories to: {output_path}")

    # Also generate JavaScript format
    js_output = Path("/home/ccd/bg/go-lang/webserver/detected_coords.js")
    with open(js_output, 'w') as f:
        f.write("/**\n")
        f.write(" * Auto-detected territory coordinates from OCR\n")
        f.write(f" * Image dimensions: {img_width}x{img_height}\n")
        f.write(" * Coordinates are percentages (0-100)\n")
        f.write(" */\n\n")
        f.write("const DETECTED_COORDS = {\n")

        for name in sorted(territories.keys()):
            coords = territories[name]
            # Estimate radius based on text length (rough heuristic)
            radius = min(3.0, max(1.5, len(name) * 0.15))
            f.write(f'    "{name}": {{ x: {coords["x"]}, y: {coords["y"]}, radius: {radius} }},\n')

        f.write("};\n")

    print(f"✓ Saved JavaScript coordinates to: {js_output}")

if __name__ == "__main__":
    main()

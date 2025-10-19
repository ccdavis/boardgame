#!/usr/bin/env python3
"""
Enhanced territory coordinate extraction using multiple OCR passes with image preprocessing.
"""

# /// script
# dependencies = [
#   "pillow",
#   "easyocr",
#   "opencv-python",
#   "numpy",
# ]
# ///

import json
import cv2
import numpy as np
from PIL import Image
import easyocr
from pathlib import Path

def preprocess_for_ocr(img):
    """Apply preprocessing to enhance text detection."""
    # Convert to grayscale
    gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)

    # Try multiple preprocessing approaches
    processed_images = []

    # 1. Original grayscale
    processed_images.append(('original', gray))

    # 2. Increase contrast
    clahe = cv2.createCLAHE(clipLimit=3.0, tileGridSize=(8,8))
    contrast_enhanced = clahe.apply(gray)
    processed_images.append(('contrast', contrast_enhanced))

    # 3. Threshold - light text on dark
    _, thresh_light = cv2.threshold(gray, 150, 255, cv2.THRESH_BINARY)
    processed_images.append(('light_text', thresh_light))

    # 4. Threshold - dark text on light
    _, thresh_dark = cv2.threshold(gray, 100, 255, cv2.THRESH_BINARY_INV)
    processed_images.append(('dark_text', thresh_dark))

    # 5. Adaptive threshold
    adaptive = cv2.adaptiveThreshold(gray, 255, cv2.ADAPTIVE_THRESH_GAUSSIAN_C,
                                     cv2.THRESH_BINARY, 11, 2)
    processed_images.append(('adaptive', adaptive))

    return processed_images

def extract_with_easyocr(image_path):
    """Use EasyOCR to detect text on the map."""
    print(f"Loading image: {image_path}")

    # Load image
    img = cv2.imread(str(image_path))
    if img is None:
        raise ValueError(f"Could not load image: {image_path}")

    height, width = img.shape[:2]
    print(f"Image dimensions: {width}x{height}")

    # Initialize EasyOCR reader (English only for speed)
    print("Initializing EasyOCR...")
    reader = easyocr.Reader(['en'], gpu=False)

    # Get preprocessed versions
    processed_images = preprocess_for_ocr(img)

    all_detections = {}

    # Try OCR on each preprocessed version
    for name, processed in processed_images:
        print(f"\nProcessing with {name} preprocessing...")

        # Run OCR
        results = reader.readtext(processed, detail=1, paragraph=False)

        print(f"  Detected {len(results)} text regions")

        for (bbox, text, conf) in results:
            text = text.strip()

            # Skip very short or low-confidence text
            if len(text) < 2 or conf < 0.3:
                continue

            # Calculate center of bounding box
            bbox_array = np.array(bbox)
            center_x = int(np.mean(bbox_array[:, 0]))
            center_y = int(np.mean(bbox_array[:, 1]))

            # Calculate percentage coordinates
            x_pct = (center_x / width) * 100
            y_pct = (center_y / height) * 100

            # If we've seen this text before, keep the higher confidence one
            if text in all_detections:
                if conf > all_detections[text]['confidence']:
                    all_detections[text] = {
                        'x': round(x_pct, 2),
                        'y': round(y_pct, 2),
                        'pixel_x': center_x,
                        'pixel_y': center_y,
                        'confidence': round(conf, 3),
                        'method': name
                    }
            else:
                all_detections[text] = {
                    'x': round(x_pct, 2),
                    'y': round(y_pct, 2),
                    'pixel_x': center_x,
                    'pixel_y': center_y,
                    'confidence': round(conf, 3),
                    'method': name
                }

    return all_detections, width, height

def main():
    map_path = Path("/home/ccd/bg/go-lang/webserver/static/images/map.jpg")

    if not map_path.exists():
        print(f"Error: Map image not found at {map_path}")
        return

    # Extract coordinates
    territories, img_width, img_height = extract_with_easyocr(map_path)

    print(f"\n{'='*80}")
    print(f"Found {len(territories)} unique territory labels:")
    print('='*80)

    # Sort by name for easier reading
    for name in sorted(territories.keys()):
        coords = territories[name]
        print(f"{name:35s} -> ({coords['pixel_x']:4d}, {coords['pixel_y']:4d}) = " +
              f"({coords['x']:6.2f}%, {coords['y']:6.2f}%) " +
              f"conf={coords['confidence']:.2f} [{coords['method']}]")

    # Save to JSON file
    output_path = Path("/home/ccd/bg/go-lang/webserver/detected_territories.json")
    with open(output_path, 'w') as f:
        json.dump({
            'image_dimensions': {'width': img_width, 'height': img_height},
            'territories': territories
        }, f, indent=2)

    print(f"\n✓ Saved detected territories to: {output_path}")

    # Generate JavaScript format
    js_output = Path("/home/ccd/bg/go-lang/webserver/detected_coords.js")
    with open(js_output, 'w') as f:
        f.write("/**\n")
        f.write(" * Auto-detected territory coordinates using EasyOCR\n")
        f.write(f" * Image dimensions: {img_width}x{img_height}\n")
        f.write(" * Coordinates are percentages (0-100)\n")
        f.write(f" * Detected {len(territories)} territories\n")
        f.write(" */\n\n")
        f.write("const DETECTED_COORDS = {\n")

        for name in sorted(territories.keys()):
            coords = territories[name]
            # Estimate radius based on confidence and text length
            radius = min(4.0, max(1.5, len(name) * 0.2))
            f.write(f'    "{name}": {{ x: {coords["x"]}, y: {coords["y"]}, radius: {radius} }},\n')

        f.write("};\n\n")
        f.write("if (typeof window !== 'undefined') {\n")
        f.write("    window.DETECTED_COORDS = DETECTED_COORDS;\n")
        f.write("}\n")

    print(f"✓ Saved JavaScript coordinates to: {js_output}")

    # Create a visualization
    print(f"\nCreating visualization...")
    img = cv2.imread(str(map_path))
    for name, coords in territories.items():
        cv2.circle(img, (coords['pixel_x'], coords['pixel_y']), 10, (0, 255, 0), 2)
        cv2.putText(img, name[:10], (coords['pixel_x'] + 15, coords['pixel_y']),
                    cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 255, 0), 1)

    viz_path = Path("/home/ccd/bg/go-lang/webserver/screenshots/detected_labels_viz.jpg")
    cv2.imwrite(str(viz_path), img)
    print(f"✓ Saved visualization to: {viz_path}")

if __name__ == "__main__":
    main()

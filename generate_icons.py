#!/usr/bin/env python3
"""Generate Android vector drawables from wireframe SVG icons"""

import os
import re

# Icon definitions extracted from wireframe
ICONS = {
    "ic_bell": '<path d="M6 9a6 6 0 0 1 12 0c0 5 2 6 2 6H4s2-1 2-6"/><path d="M10 19a2 2 0 0 0 4 0"/>',
    "ic_search": '<circle cx="11" cy="11" r="6"/><path d="m20 20-4.2-4.2"/>',
    "ic_pin": '<path d="M12 21s-6.5-5.6-6.5-10.5a6.5 6.5 0 1 1 13 0C18.5 15.4 12 21 12 21z"/><circle cx="12" cy="10.5" r="2.2"/>',
    "ic_globe": '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c2.5 2.6 3.8 5.7 3.8 9S14.5 18.4 12 21c-2.5-2.6-3.8-5.7-3.8-9S9.5 5.6 12 3z"/>',
    "ic_check": '<path d="M4.5 12.5 10 18 19.5 6.5"/>',
    "ic_clock": '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3.5 2"/>',
    "ic_plus": '<path d="M12 5v14M5 12h14"/>',
    "ic_star": '<path d="m12 3 2.7 5.8 6.3.7-4.7 4.3 1.3 6.2-5.6-3.2L6.4 20l1.3-6.2L3 9.5l6.3-.7z"/>',
    "ic_anchor": '<circle cx="12" cy="5.5" r="2.5"/><path d="M12 8v13M5 12H2c0 5 4.5 9 10 9s10-4 10-9h-3"/>',
    "ic_waves": '<path d="M2 8c2.5-2 5-2 7.5 0S15 10 17.5 8 21 6.5 22 7M2 14c2.5-2 5-2 7.5 0s3 2 5.5 0 5-2 7-1.5M2 20c2.5-2 5-2 7.5 0s3 2 5.5 0 5-2 7-1.5"/>',
    "ic_doc": '<path d="M6 3h8l4 4v14H6z"/><path d="M14 3v4h4M9 12h6M9 16h6"/>',
    "ic_scan": '<path d="M4 8V5a1 1 0 0 1 1-1h3M16 4h3a1 1 0 0 1 1 1v3M20 16v3a1 1 0 0 1-1 1h-3M8 20H5a1 1 0 0 1-1-1v-3M3 12h18"/>',
    "ic_sync": '<path d="M20 12a8 8 0 0 1-14.6 4.5M4 12a8 8 0 0 1 14.6-4.5"/><path d="M18.5 3v4.5H14M5.5 21v-4.5H10"/>',
    "ic_send": '<path d="M21 3 10 14M21 3l-7 18-4-7-7-4z"/>',
    "ic_boat": '<path d="M4 16h16l-2 4H6z"/><path d="M12 3v13M12 5c3 1 5 3 6 6h-6M12 8c-2 .8-3.5 2.5-4 5h4"/>',
    "ic_card": '<rect x="3" y="6" width="18" height="13" rx="2"/><path d="M3 10.5h18M7 15h4"/>',
    "ic_db": '<ellipse cx="12" cy="5.5" rx="8" ry="2.8"/><path d="M4 5.5V18.5c0 1.5 3.6 2.8 8 2.8s8-1.3 8-2.8V5.5"/><path d="M4 12c0 1.5 3.6 2.8 8 2.8s8-1.3 8-2.8"/>',
    "ic_link": '<path d="M10 14a4 4 0 0 0 6 .4l3-3a4 4 0 0 0-5.6-5.6l-1.7 1.7M14 10a4 4 0 0 0-6-.4l-3 3a4 4 0 0 0 5.6 5.6l1.7-1.7"/>',
    "ic_tts": '<path d="M4 10v4h3l5 4V6l-5 4z"/><path d="M15.5 8.5a5 5 0 0 1 0 7M18 6a8.5 8.5 0 0 1 0 12"/>',
    "ic_arr": '<path d="M4 12h15M13 6l6 6-6 6"/>',
    "ic_bat": '<rect x="3" y="8" width="16" height="8" rx="2"/><path d="M21 11v2"/><rect x="5.5" y="10.5" width="8" height="3" rx="1" fill="currentColor" stroke="none"/>',
    "ic_sig": '<path d="M5 18v-3M10 18v-6M15 18V8M20 18V4" stroke-width="2.4"/>',
    "ic_co2": '<path d="M7 17a4 4 0 1 1 .6-7.97A5 5 0 0 1 17.3 8.7 3.5 3.5 0 0 1 17 17z"/><path d="M9 20h6"/>',
    "ic_gift": '<rect x="4" y="10" width="16" height="10" rx="1.5"/><path d="M12 10v10M4 14h16M12 10S9 9.5 8 7.5C7.2 5.8 9 4 10.5 5c1.4.9 1.5 5 1.5 5zm0 0s3-.5 4-2.5C16.8 5.8 15 4 13.5 5c-1.4.9-1.5 5-1.5 5z"/>',
    "ic_sat": '<rect x="9" y="9" width="6" height="6" rx="1" transform="rotate(45 12 12)"/><path d="M3.5 8.5 8 4M16 20l4.5-4.5M2 13l2.5 2.5M19.5 8.5 22 6"/><path d="M14 18c2 2 5 2 7 0M6 10C4 8 4 5 6 3"/>',
    "ic_shield": '<path d="M12 3l7 2.8V11c0 4.8-3 8.2-7 10-4-1.8-7-5.2-7-10V5.8z"/><path d="M9 12l2 2 4-4"/>',
    "ic_chart": '<path d="M4 20V10M10 20V4M16 20v-7M21 20H3"/>',
    "ic_cam": '<path d="M4 8h3l2-2.5h6L17 8h3a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V9a1 1 0 0 1 1-1z"/><circle cx="12" cy="14" r="3.5"/>',
    "ic_mic": '<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3"/>',
}

TEMPLATE = '''<vector xmlns:android="http://schemas.android.com/apk/res/android"
    android:width="24dp"
    android:height="24dp"
    android:viewportWidth="24"
    android:viewportHeight="24">
    {paths}
</vector>
'''

def convert_svg_to_android(svg_content):
    """Convert SVG path elements to Android vector format"""
    paths = []
    
    # Extract individual path/circle/rect/ellipse elements
    elements = re.findall(r'<(path|circle|rect|ellipse)[^>]*>', svg_content)
    
    for element_match in re.finditer(r'<(path|circle|rect|ellipse)([^>]*)>', svg_content):
        element_type = element_match.group(1)
        attributes = element_match.group(2)
        
        # Check if this element has fill="currentColor" (should be filled, not stroked)
        is_filled = 'fill="currentColor"' in attributes
        
        if element_type == 'path':
            # Extract d attribute
            d_match = re.search(r'd="([^"]*)"', attributes)
            if d_match:
                path_data = d_match.group(1)
                
                # Check for special stroke-width
                stroke_width = "1.8"
                sw_match = re.search(r'stroke-width="([^"]*)"', attributes)
                if sw_match:
                    stroke_width = sw_match.group(1)
                
                if is_filled:
                    paths.append(f'''    <path
        android:pathData="{path_data}"
        android:fillColor="@android:color/black"
        android:strokeWidth="0"/>''')
                else:
                    paths.append(f'''    <path
        android:pathData="{path_data}"
        android:strokeColor="@android:color/black"
        android:strokeWidth="{stroke_width}"
        android:strokeLineCap="round"
        android:strokeLineJoin="round"
        android:fillColor="@android:color/transparent"/>''')
        
        elif element_type == 'circle':
            cx_match = re.search(r'cx="([^"]*)"', attributes)
            cy_match = re.search(r'cy="([^"]*)"', attributes)
            r_match = re.search(r'r="([^"]*)"', attributes)
            
            if cx_match and cy_match and r_match:
                cx, cy, r = cx_match.group(1), cy_match.group(1), r_match.group(1)
                
                if is_filled:
                    paths.append(f'''    <circle
        android:cx="{cx}"
        android:cy="{cy}"
        android:radius="{r}"
        android:fillColor="@android:color/black"
        android:strokeWidth="0"/>''')
                else:
                    paths.append(f'''    <circle
        android:cx="{cx}"
        android:cy="{cy}"
        android:radius="{r}"
        android:strokeColor="@android:color/black"
        android:strokeWidth="1.8"
        android:strokeLineCap="round"
        android:strokeLineJoin="round"
        android:fillColor="@android:color/transparent"/>''')
        
        elif element_type == 'rect':
            x_match = re.search(r'x="([^"]*)"', attributes)
            y_match = re.search(r'y="([^"]*)"', attributes)
            w_match = re.search(r'width="([^"]*)"', attributes)
            h_match = re.search(r'height="([^"]*)"', attributes)
            rx_match = re.search(r'rx="([^"]*)"', attributes)
            
            if x_match and y_match and w_match and h_match:
                x, y, w, h = x_match.group(1), y_match.group(1), w_match.group(1), h_match.group(1)
                rx = rx_match.group(1) if rx_match else "0"
                
                # Check for transform
                transform = ""
                transform_match = re.search(r'transform="([^"]*)"', attributes)
                if transform_match:
                    # Note: Android vector drawables don't support transform directly
                    # This would need manual conversion
                    pass
                
                if is_filled:
                    paths.append(f'''    <path
        android:pathData="M{x},{y}h{w}v{h}h-{w}z"
        android:fillColor="@android:color/black"
        android:strokeWidth="0"/>''')
                else:
                    # Convert rect to path for better control
                    if float(rx) > 0:
                        # Rounded rect - approximate
                        paths.append(f'''    <path
        android:pathData="M{x},{float(y)+float(rx)}a{rx},{rx} 0,0 1,{rx},-{rx}h{float(w)-2*float(rx)}a{rx},{rx} 0,0 1,{rx},{rx}v{float(h)-2*float(rx)}a{rx},{rx} 0,0 1,-{rx},{rx}h-{float(w)-2*float(rx)}a{rx},{rx} 0,0 1,-{rx},-{rx}z"
        android:strokeColor="@android:color/black"
        android:strokeWidth="1.8"
        android:fillColor="@android:color/transparent"/>''')
                    else:
                        paths.append(f'''    <path
        android:pathData="M{x},{y}h{w}v{h}h-{w}z"
        android:strokeColor="@android:color/black"
        android:strokeWidth="1.8"
        android:strokeLineCap="round"
        android:strokeLineJoin="round"
        android:fillColor="@android:color/transparent"/>''')
        
        elif element_type == 'ellipse':
            cx_match = re.search(r'cx="([^"]*)"', attributes)
            cy_match = re.search(r'cy="([^"]*)"', attributes)
            rx_match = re.search(r'rx="([^"]*)"', attributes)
            ry_match = re.search(r'ry="([^"]*)"', attributes)
            
            if cx_match and cy_match and rx_match and ry_match:
                cx, cy, rx, ry = cx_match.group(1), cy_match.group(1), rx_match.group(1), ry_match.group(1)
                # Android doesn't have ellipse, convert to path
                paths.append(f'''    <path
        android:pathData="M{float(cx)-float(rx)},{cy}a{rx},{ry} 0,1 0,{float(rx)*2} 0a{rx},{ry} 0,1 0,-{float(rx)*2} 0"
        android:strokeColor="@android:color/black"
        android:strokeWidth="1.8"
        android:strokeLineCap="round"
        android:strokeLineJoin="round"
        android:fillColor="@android:color/transparent"/>''')
    
    return '\n'.join(paths)

def main():
    output_dir = "/Users/Komarov/Documents/BahariMRV/android/app/src/main/res/drawable"
    os.makedirs(output_dir, exist_ok=True)
    
    for icon_name, svg_content in ICONS.items():
        android_paths = convert_svg_to_android(svg_content)
        xml_content = TEMPLATE.format(paths=android_paths)
        
        output_path = os.path.join(output_dir, f"{icon_name}.xml")
        with open(output_path, 'w') as f:
            f.write(xml_content)
        
        print(f"Created {icon_name}.xml")
    
    print(f"\nGenerated {len(ICONS)} icon files")

if __name__ == "__main__":
    main()

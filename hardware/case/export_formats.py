#!/usr/bin/env python3
"""acrylic_svg.py が書き出した図面(SVG)から、DXF と PDF を作る。
使い方: python3 export_formats.py takoyaki8_acrylic_set1.svg
        → 同じ名前で .dxf と .pdf ができる(PDF は Inkscape が必要。なければ DXF だけ作る)
DXF: 単位は mm。向きは SVG と同じ見た目(y を反転して書く)。
     レイヤー CUT(赤。切る線)と ENGRAVE(彫刻の輪郭)。円と円弧は、近似せずに書く。
必要なパッケージ: なし(標準ライブラリだけ)
"""
import math, re, shutil, subprocess, sys
import xml.etree.ElementTree as ET
NS = '{http://www.w3.org/2000/svg}'


def parse_d(d):
    """M L C A Z のパス → [(頂点の列, 閉じているか)]。頂点は [x, y, bulge]。
    bulge は、次の頂点までの円弧の膨らみ(DXF 用。0 なら直線)。曲線(C)は折れ線に直す"""
    tok = re.findall(r'[MLACZ]|-?\d*\.?\d+(?:e-?\d+)?', d)
    i, subs, cur, p = 0, [], None, (0.0, 0.0)
    while i < len(tok):
        c = tok[i]; i += 1
        if c == 'M':
            if cur: subs.append((cur, False))
            p = (float(tok[i]), float(tok[i + 1])); i += 2; cur = [[p[0], p[1], 0.0]]
        elif c == 'L':
            while i < len(tok) and tok[i] not in 'MLACZ':
                p = (float(tok[i]), float(tok[i + 1])); i += 2; cur.append([p[0], p[1], 0.0])
        elif c == 'C':
            while i < len(tok) and tok[i] not in 'MLACZ':
                v = [float(t) for t in tok[i:i + 6]]; i += 6
                for k in range(1, 17):
                    u = k / 16
                    x = (1-u)**3*p[0] + 3*(1-u)**2*u*v[0] + 3*(1-u)*u*u*v[2] + u**3*v[4]
                    y = (1-u)**3*p[1] + 3*(1-u)**2*u*v[1] + 3*(1-u)*u*u*v[3] + u**3*v[5]
                    cur.append([x, y, 0.0])
                p = (v[4], v[5])
        elif c == 'A':
            r, _, _, large, sweep, x, y = [float(t) for t in tok[i:i + 7]]; i += 7
            chord = math.hypot(x - p[0], y - p[1])
            half = min(1.0, chord / (2 * r))
            theta = 2 * math.asin(half)
            if int(large): theta = 2 * math.pi - theta
            # SVG の sweep=1 は、画面で時計回り。y を反転して書いても、見た目は同じ(時計回り)なので、DXF では bulge が負になる
            cur[-1][2] = math.tan(theta / 4) * (-1 if int(sweep) else 1)
            p = (x, y); cur.append([x, y, 0.0])
        elif c == 'Z':
            if cur:
                # 最後の頂点が最初の頂点と同じなら、重ねて書かない
                if len(cur) > 1 and abs(cur[-1][0] - cur[0][0]) < 1e-6 and abs(cur[-1][1] - cur[0][1]) < 1e-6:
                    cur.pop()
                subs.append((cur, True)); cur = None
    if cur: subs.append((cur, False))
    return subs


def read_svg(path):
    root = ET.parse(path).getroot()
    H = float(root.get('viewBox').split()[3])
    layers = {}
    for g in root.iter(NS + 'g'):
        if g.get('id') in ('cut', 'engrave'):
            layers[g.get('id')] = list(g)
    return H, layers


def g(code, val): return '%d\n%s\n' % (code, val)


def make_dxf(H, layers):
    out = [g(0, 'SECTION') + g(2, 'HEADER') + g(9, '$ACADVER') + g(1, 'AC1009') + g(0, 'ENDSEC')]
    out.append(g(0, 'SECTION') + g(2, 'TABLES') + g(0, 'TABLE') + g(2, 'LAYER') + g(70, 2) +
               g(0, 'LAYER') + g(2, 'CUT') + g(70, 0) + g(62, 1) + g(6, 'CONTINUOUS') +
               g(0, 'LAYER') + g(2, 'ENGRAVE') + g(70, 0) + g(62, 7) + g(6, 'CONTINUOUS') +
               g(0, 'ENDTAB') + g(0, 'ENDSEC'))
    ent = [g(0, 'SECTION') + g(2, 'ENTITIES')]
    for key, layer in (('cut', 'CUT'), ('engrave', 'ENGRAVE')):
        for e in layers.get(key, []):
            tag = e.tag.replace(NS, '')
            if tag == 'circle':
                ent.append(g(0, 'CIRCLE') + g(8, layer) + g(10, '%.4f' % float(e.get('cx'))) +
                           g(20, '%.4f' % (H - float(e.get('cy')))) + g(30, 0) + g(40, '%.4f' % float(e.get('r'))))
            elif tag == 'path':
                for verts, closed in parse_d(e.get('d')):
                    s = g(0, 'POLYLINE') + g(8, layer) + g(66, 1) + g(70, 1 if closed else 0)
                    for x, y, b in verts:
                        s += g(0, 'VERTEX') + g(8, layer) + g(10, '%.4f' % x) + g(20, '%.4f' % (H - y)) + g(30, 0)
                        if abs(b) > 1e-9: s += g(42, '%.6f' % b)
                    ent.append(s + g(0, 'SEQEND') + g(8, layer))
    ent.append(g(0, 'ENDSEC') + g(0, 'EOF'))
    return ''.join(out + ent)


def main():
    if len(sys.argv) != 2 or not sys.argv[1].endswith('.svg'):
        raise SystemExit('使い方: python3 export_formats.py 図面.svg')
    svg = sys.argv[1]; base = svg[:-4]
    H, layers = read_svg(svg)
    open(base + '.dxf', 'w', encoding='ascii', newline='\n').write(make_dxf(H, layers))
    n = sum(len(v) for v in layers.values())
    print('書き出し:', base + '.dxf', '(図形 %d 個。単位 mm)' % n)
    ink = shutil.which('inkscape')
    if ink:
        subprocess.run([ink, svg, '--export-type=pdf', '--export-filename=' + base + '.pdf'], check=True)
        print('書き出し:', base + '.pdf')
    else:
        print('inkscape が見つからないので、PDF は作りませんでした(Inkscape で SVG を開いて、PDF で保存してください)')


if __name__ == '__main__':
    main()

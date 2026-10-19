#!/usr/bin/env python3
"""takoyaki8 アクリル図面(Anymany入稿用SVG)の生成スクリプト。
座標は基板座標(基板の左上が原点、x右、y下、mm)。上から見た向きで部品を定義し、
彫刻は「基板に向いた内側の面」なので、ボトムは上面(上から見た向き)、カバーは下面(左右反転)を上にして書き出す。
必要なパッケージ: pip install numpy pillow opencv-python-headless matplotlib
使い方: python3 acrylic_svg.py --pcb takoyaki8.kicad_pcb --sheet 15x30 --out acrylic_15x30.svg
"""
import argparse, math, re, sys
import numpy as np
from PIL import Image, ImageDraw
import cv2
from matplotlib.textpath import TextPath
from matplotlib.font_manager import FontProperties

# ================= パラメータ(基板が届いたらここを書き換える) =================
BOARD_W, BOARD_H, BOARD_R = 99.9, 40.05, 1.0
PCB_ORIGIN = (100.0, 60.0)               # KiCad座標 → 基板座標の原点
HOLE_SCREW_D = 2.2                        # ネジ穴 φ(実績あり)
HOLES_COVER = [(4.25, 25.65), (4.25, 36.55), (22.40, 37.00)]
HOLES_TOP   = [(42.5, 3.0), (80.6, 3.0), (44.0, 37.2), (82.1, 37.2)]
BOOTRESET   = [(9.13, 15.90), (18.26, 15.90)]     # 位置は推定 ±0.3(実物で確認)
NOTCH_X     = (6.7, 20.5)                         # USB側の切り欠き(幅)。深さは種類ごと
COVER_GAP   = 0.5                                 # カバーとPCBのフチの空き(上・下・左をそろえる)
COVER_BOX   = (COVER_GAP, COVER_GAP, 24.5, BOARD_H - COVER_GAP)   # x0,y0,x右(基準),y1
COVER_R_LEFT = BOARD_R - COVER_GAP                # 左の角R: PCBの角R1.0と同心(空きが一定)= 0.5
FROST_MARGIN = 2.0                                # 彫刻面の外周の余白
FROST_CORNER_R = 1.0
RING_R = 2.2                                      # 穴のまわりの透明な半径
VA = (10.11, 8.31, 17.69, 32.69)                  # OLED表示領域
WIN_MARGIN = 0.35                                 # 窓の余白
COVER_TEXT = dict(scale=1.0, center=(6.13, 13.0))   # ロゴ(TakoyaKeeb_logo.svg 19.0×3.2mm)は等倍  # 縦書きの抜き文字
COVER_LABEL = dict(size=3.6, center=(13.2, 35.3))
PLATE_TEXT_SCALE = 4.2
LEDX = [33.5, 52.55, 71.6, 90.65]; LEDY = 22.0
PLATE_LABEL = dict(size=4.2, center=(14.0, 31.0))
# 試作の組み合わせ
BOOT_D = [2.5, 3.0]; NOTCH_DEPTH = [0.0, 3.0, 6.0]; COVER_XR = [24.5, 25.0, 25.5]; COVER_COPIES = 3
GAP, MARGIN = 3.0, 3.0                             # Anymanyのルール
PT = 25.4 / 72                                     # 1pt = 0.3528mm

# ================= ロゴ(文字)の取り出し =================
def _parse(s):
    tok = re.findall(r'\(|\)|"(?:\\.|[^"\\])*"|[^\s()]+', s); pos = 0
    def rd():
        nonlocal pos
        t = tok[pos]
        if t == '(':
            pos += 1; l = []
            while tok[pos] != ')': l.append(rd())
            pos += 1; return l
        pos += 1; return t[1:-1] if t.startswith('"') else t
    return rd()
def load_logo_text(pcb):
    """基板の裏面シルクのロゴから、文字(TakoyaKeeb)の多角形を『読める向き』で返す"""
    t = _parse(open(pcb, encoding='utf-8').read())
    for fp in t:
        if isinstance(fp, list) and fp and fp[0] == 'footprint' and fp[1] == 'takoyaki8:TakoyaKeeb_logo':
            at = [y for y in fp if isinstance(y, list) and y[0] == 'at'][0]
            ax, ay = float(at[1]), float(at[2]); a = math.radians(float(at[3]) if len(at) > 3 else 0)
            c, s = math.cos(a), math.sin(a)
            polys = []
            for y in fp:
                if isinstance(y, list) and y[0] == 'fp_poly':
                    pts = [(float(p[1]), float(p[2])) for p in [z for z in y if z[0] == 'pts'][0][1:]]
                    polys.append([(ax + c*x - s*yy - PCB_ORIGIN[0], ay + s*x + c*yy - PCB_ORIGIN[1]) for x, yy in pts])
            xs = [x for q in polys for x, _ in q]; cx = (min(xs) + max(xs)) / 2
            reading = [[(2*cx - x, y) for x, y in q] for q in polys]      # 裏面ロゴ→読める向き
            return [q for q in reading if min(y for _, y in q) >= 32.2]   # 文字だけ(マスコットは y<32.2)
    raise SystemExit('ロゴのフットプリントが見つかりません')

def bbox(polys):
    X = [x for q in polys for x, _ in q]; Y = [y for q in polys for _, y in q]
    return min(X), min(Y), max(X), max(Y)
def place(polys, scale, cx, cy, mode):
    """mode: top=上から読める / back=下から読める(上面図では左右反転) / up90=上から見て下→上に縦書き"""
    x0, y0, x1, y1 = bbox(polys); mu, mv = (x0+x1)/2, (y0+y1)/2; out = []
    for q in polys:
        r = []
        for u, v in q:
            du, dv = (u-mu)*scale, (v-mv)*scale
            r.append({'top': (cx+du, cy+dv), 'back': (cx-du, cy+dv), 'up90': (cx+dv, cy-du)}[mode])
        out.append(r)
    return out
def label_polys(s, size):
    tp = TextPath((0, 0), s, size=size, prop=FontProperties(family='DejaVu Sans', weight='bold'))
    return [[(x, -y) for x, y in poly[:-1]] for poly in tp.to_polygons() if len(poly) > 3]

# ================= 図形 =================
def fmt(v): return ('%.3f' % v).rstrip('0').rstrip('.')
def pd(polys): return ' '.join('M' + ' L'.join('%s,%s' % (fmt(x), fmt(y)) for x, y in q) + ' Z' for q in polys)
def plate_outline_d(ox, oy, notch):
    w, h, r = BOARD_W, BOARD_H, BOARD_R; X = lambda x: fmt(ox+x); Y = lambda y: fmt(oy+y)
    s = 'M %s,%s L %s,%s ' % (X(r), Y(0), X(NOTCH_X[0]), Y(0))
    if notch > 0:
        s += 'L %s,%s L %s,%s L %s,%s ' % (X(NOTCH_X[0]), Y(notch), X(NOTCH_X[1]), Y(notch), X(NOTCH_X[1]), Y(0))
    s += 'L %s,%s A %s %s 0 0 1 %s,%s L %s,%s A %s %s 0 0 1 %s,%s L %s,%s A %s %s 0 0 1 %s,%s L %s,%s A %s %s 0 0 1 %s,%s Z' % (
        X(w-r), Y(0), fmt(r), fmt(r), X(w), Y(r), X(w), Y(h-r), fmt(r), fmt(r), X(w-r), Y(h),
        X(r), Y(h), fmt(r), fmt(r), X(0), Y(h-r), X(0), Y(r), fmt(r), fmt(r), X(r), Y(0))
    return s
def cover_size(xr): return xr - COVER_BOX[0], COVER_BOX[3] - COVER_BOX[1]
def cover_outline_d(ox, oy, xr, slot=False):
    """彫刻面を上にした向き(左右反転)なので、角Rは右側に付く。
    slot=True: 右下の穴(22.4, 37.0)を、取り付け状態で右(キー側)に開く溝にする(「U」を時計回りに90度回した形)。
    この向きでは溝は左(x=0)に開く。"""
    w, h = cover_size(xr); R = COVER_R_LEFT; X = lambda x: fmt(ox+x); Y = lambda y: fmt(oy+y)
    if slot:
        lx = xr - HOLES_COVER[2][0]; ly = HOLES_COVER[2][1] - COVER_BOX[1]; r = HOLE_SCREW_D/2
        return ('M %s,%s L %s,%s A %s %s 0 0 1 %s,%s L %s,%s A %s %s 0 0 1 %s,%s L %s,%s L %s,%s L %s,%s A %s %s 0 0 0 %s,%s L %s,%s Z' %
                (X(0), Y(0), X(w-R), Y(0), fmt(R), fmt(R), X(w), Y(R), X(w), Y(h-R), fmt(R), fmt(R), X(w-R), Y(h),
                 X(0), Y(h), X(0), Y(ly+r), X(lx), Y(ly+r), fmt(r), fmt(r), X(lx), Y(ly-r), X(0), Y(ly-r)))
    return ('M %s,%s L %s,%s A %s %s 0 0 1 %s,%s L %s,%s A %s %s 0 0 1 %s,%s L %s,%s Z' %
            (X(0), Y(0), X(w-R), Y(0), fmt(R), fmt(R), X(w), Y(R), X(w), Y(h-R), fmt(R), fmt(R), X(w-R), Y(h), X(0), Y(h)))

def rrect_pts(x0, y0, x1, y1, r, n=16):
    pts = []
    for cx, cy, a0 in ((x0+r, y0+r, 180), (x1-r, y0+r, 270), (x1-r, y1-r, 0), (x0+r, y1-r, 90)):
        for i in range(n+1):
            a = math.radians(a0 + 90*i/n); pts.append((cx + r*math.cos(a), cy + r*math.sin(a)))
    return pts
def circ_pts(cx, cy, r, n=96): return [(cx + r*math.cos(2*math.pi*i/n), cy + r*math.sin(2*math.pi*i/n)) for i in range(n)]

def cover_frost_polys(xr, text_polys, label):
    """カバーの彫刻面(上から見た向き)= 余白を除いた全面 − 窓 − 穴のまわり − 抜き文字。ラスターで引き算して輪郭をたどる"""
    res = 0.005; x0, y0, _, y1 = COVER_BOX; bx0, by0 = x0 - 1, y0 - 1
    W = int((xr - x0 + 2) / res); H = int((y1 - y0 + 2) / res)
    def mask(q):
        im = Image.new('1', (W, H), 0)
        ImageDraw.Draw(im).polygon([((x-bx0)/res, (y-by0)/res) for x, y in q], fill=1); return np.array(im, bool)
    m = mask(rrect_pts(x0+FROST_MARGIN, y0+FROST_MARGIN, xr-FROST_MARGIN, y1-FROST_MARGIN, FROST_CORNER_R))
    v = VA; m &= ~mask([(v[0]-WIN_MARGIN, v[1]-WIN_MARGIN), (v[2]+WIN_MARGIN, v[1]-WIN_MARGIN), (v[2]+WIN_MARGIN, v[3]+WIN_MARGIN), (v[0]-WIN_MARGIN, v[3]+WIN_MARGIN)])
    for hx, hy in HOLES_COVER: m &= ~mask(circ_pts(hx, hy, RING_R))
    for group in (text_polys, label):
        ink = np.zeros_like(m)
        for q in group: ink ^= mask(q)
        m &= ~ink
    cs, hier = cv2.findContours(m.astype(np.uint8), cv2.RETR_CCOMP, cv2.CHAIN_APPROX_NONE)
    out = []
    for c in cs:
        a = cv2.approxPolyDP(c, 1.0, True)[:, 0, :]
        if len(a) >= 3: out.append([(bx0 + (px + 0.5)*res, by0 + (py + 0.5)*res) for px, py in a])
    return out


# ================= イラスト(SVG)の取り込み =================
def _mat(s):
    m = re.match(r'matrix\(([^)]*)\)', s or '')
    if m: a, b, c, d, e, f = [float(v) for v in re.split(r'[ ,]+', m.group(1).strip())]; return (a, b, c, d, e, f)
    m = re.match(r'translate\(([^)]*)\)', s or '')
    if m: v = [float(t) for t in re.split(r'[ ,]+', m.group(1).strip())]; return (1, 0, 0, 1, v[0], v[1] if len(v) > 1 else 0)
    return (1, 0, 0, 1, 0, 0)
def _mul(m, n):   # m を外側、n を内側にして合成
    a, b, c, d, e, f = m; A, B, C, D, E, F = n
    return (a*A + c*B, b*A + d*B, a*C + c*D, b*C + d*D, a*E + c*F + e, b*E + d*F + f)
def _ap(m, p): return (m[0]*p[0] + m[2]*p[1] + m[4], m[1]*p[0] + m[3]*p[1] + m[5])
def _inv(m):
    a, b, c, d, e, f = m; det = a*d - b*c
    return (d/det, -b/det, -c/det, a/det, (c*f - d*e)/det, (b*e - a*f)/det)
def _subpaths(d):
    """M/L/C/Z(絶対座標)のパスを、[(点, 'M'|'L'|'C', 制御点...)] の部分パスに分ける"""
    tok = re.findall(r'[MLCZ]|-?\d*\.?\d+(?:e-?\d+)?', d); i = 0; subs = []; cur = None
    while i < len(tok):
        c = tok[i]; i += 1
        if c == 'M':
            if cur: subs.append(cur)
            cur = [('M', (float(tok[i]), float(tok[i+1])))]; i += 2
        elif c == 'L':
            while i < len(tok) and tok[i] not in 'MLCZ': cur.append(('L', (float(tok[i]), float(tok[i+1])))); i += 2
        elif c == 'C':
            while i < len(tok) and tok[i] not in 'MLCZ':
                v = [float(t) for t in tok[i:i+6]]; cur.append(('C', (v[0], v[1]), (v[2], v[3]), (v[4], v[5]))); i += 6
        elif c == 'Z':
            subs.append(cur); cur = None
    if cur: subs.append(cur)
    return subs
def _bbox_sub(subs):
    pts = [p for s in subs for seg in s for p in seg[1:]]
    return min(p[0] for p in pts), min(p[1] for p in pts), max(p[0] for p in pts), max(p[1] for p in pts)
def load_art(svgfile):
    """入稿用に描かれたSVG(下から見た向き)から、イラストのパスを『ボトムの上面図の座標』に直して返す。
    P?_outline のグループの変換と外形の位置を基準にするので、図の拡大率・向きが違っても追従する。
    戻り値: (部分パスのリスト, 報告用の情報)"""
    import xml.etree.ElementTree as ET
    root = ET.parse(svgfile).getroot(); NS = '{http://www.w3.org/2000/svg}'
    info = dict(colors=set(), opacities=set(), n_paths=0); out = []; ref = {}
    def walk(e, m, in_part):
        m = _mul(m, _mat(e.get('transform')))
        i = e.get('id') or ''
        part = in_part or bool(re.match(r'^P\d+_', i))
        if re.match(r'^P\d+_outline$', i):
            p = [c for c in e if c.tag == NS + 'path'][0]; ref['m'] = m; ref['bb'] = _bbox_sub(_subpaths(p.get('d'))); ref['id'] = i
        if e.tag == NS + 'path' and not part:
            st = e.get('style') or ''; fm = re.search(r'fill:\s*([^;]+)', st); fill = fm.group(1).strip() if fm else 'black'
            om = re.search(r'fill-opacity:\s*([\d.]+)', st)
            if fill != 'none':
                info['colors'].add(fill); info['opacities'].add(om.group(1) if om else '1'); info['n_paths'] += 1
                out.append((_subpaths(e.get('d')), m))
        for c in e: walk(c, m, part)
    walk(root, (1, 0, 0, 1, 0, 0), False)
    if 'm' not in ref: raise SystemExit('イラストSVGに P?_outline が見つかりません(基準にする外形が必要です)')
    inv = _inv(ref['m']); bx, by = ref['bb'][0], ref['bb'][1]
    to_local = lambda p, m: tuple(a - b for a, b in zip(_ap(inv, _ap(m, p)), (bx, by)))
    res = []
    for subs, m in out:
        res.append([[tuple([seg[0]] + [to_local(p, m) for p in seg[1:]]) for seg in s] for s in subs])
    info['ref'] = ref['id']; return res, info
def art_paths(art_sub, ox, oy):
    """イラストをシート座標のSVGパス文字列にする(曲線は曲線のまま)。色は黒の塗りに統一される"""
    ds = []
    for subs in art_sub:
        d = []
        for s in subs:
            for seg in s:
                pts = ' '.join('%s,%s' % (fmt(p[0]+ox), fmt(p[1]+oy)) for p in seg[1:])
                d.append('%s%s' % (seg[0], pts))
            d.append('Z')
        ds.append(' '.join(d))
    return ds

# ================= シートの組み立て =================
def build(pcb, sheet, art=None):
    art_sub = load_art(art)[0] if art else None
    W, H = {'15x30': (300, 150), '30x30': (300, 300)}[sheet]
    text = load_logo_text(pcb); cx = (LEDX[0] + LEDX[-1]) / 2
    cut, eng, items = [], [], []
    x = MARGIN; n = 0
    for ci, b in enumerate(BOOT_D):
        for ri, nd in enumerate(NOTCH_DEPTH):
            n += 1; name = 'P%d' % n; ox, oy = x, MARGIN + ri*(BOARD_H + GAP)
            cut.append((name + '_outline', 'path', plate_outline_d(ox, oy, nd)))
            for k, (hx, hy) in enumerate(HOLES_COVER + HOLES_TOP): cut.append((name + '_hole%d' % (k+1), 'circle', (ox+hx, oy+hy, HOLE_SCREW_D/2)))
            for k, (hx, hy) in enumerate(BOOTRESET): cut.append((name + '_bootreset%d' % (k+1), 'circle', (ox+hx, oy+hy, b/2)))
            t = place(text, PLATE_TEXT_SCALE, cx, LEDY, 'back')
            lb = place(label_polys(name, PLATE_LABEL['size']), 1.0, *PLATE_LABEL['center'], 'back')
            if art is None: eng.append((name + '_text', pd([[(ox+px, oy+py) for px, py in q] for q in t])))
            else:
                for k, d in enumerate(art_paths(art_sub, ox, oy)): eng.append(('%s_art%d' % (name, k+1), d))
            eng.append((name + '_label', pd([[(ox+px, oy+py) for px, py in q] for q in lb])))
            items.append(dict(name=name, kind='plate', x=ox, y=oy, w=BOARD_W, h=BOARD_H, boot=b, notch=nd))
        x += BOARD_W + GAP
    wmax = max(cover_size(xr)[0] for xr in COVER_XR)
    for ci, xr in enumerate(COVER_XR):
        w, h = cover_size(xr)
        t = place(text, COVER_TEXT['scale'], *COVER_TEXT['center'], 'up90')
        for ri in range(COVER_COPIES):
            name = 'C%d' % (ci+1); ox, oy = x + ci*(wmax + GAP), MARGIN + ri*(h + GAP)
            lb = place(label_polys(name, COVER_LABEL['size']), 1.0, *COVER_LABEL['center'], 'top')
            frost = cover_frost_polys(xr, t, lb)
            cut.append(('%s_%d_outline' % (name, ri+1), 'path', cover_outline_d(ox, oy, xr)))
            for k, (hx, hy) in enumerate(HOLES_COVER):
                cut.append(('%s_%d_hole%d' % (name, ri+1, k+1), 'circle', (ox + (xr - hx), oy + (hy - COVER_BOX[1]), HOLE_SCREW_D/2)))
            eng.append(('%s_%d_frost' % (name, ri+1), pd([[(ox + (xr - px), oy + (py - COVER_BOX[1])) for px, py in q] for q in frost])))
            items.append(dict(name='%s_%d' % (name, ri+1), kind='cover', x=ox, y=oy, w=w, h=h, xr=xr))
    L = ['<?xml version="1.0" encoding="UTF-8"?>',
         '<svg xmlns="http://www.w3.org/2000/svg" xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" width="%dmm" height="%dmm" viewBox="0 0 %d %d">' % (W, H, W, H),
         '<g id="cut" inkscape:groupmode="layer" inkscape:label="cut(赤1pt)" fill="none" stroke="#ff0000" stroke-width="%.4f">' % PT]
    for nm, kind, d in cut:
        L.append('<path id="%s" d="%s"/>' % (nm, d) if kind == 'path' else '<circle id="%s" cx="%s" cy="%s" r="%s"/>' % (nm, fmt(d[0]), fmt(d[1]), fmt(d[2])))
    L.append('</g>')
    L.append('<g id="engrave" inkscape:groupmode="layer" inkscape:label="engrave(黒の塗り)" fill="#000000" stroke="none" fill-rule="evenodd">')
    for nm, d in eng: L.append('<path id="%s" d="%s"/>' % (nm, d))
    L += ['</g>', '</svg>']
    return '\n'.join(L), W, H, items


# ================= ロゴ(Inkscapeで作ったSVG)の取り込み(カバーの抜き文字) =================
def load_logo_svg(svgfile, curve_steps=24):
    """Inkscapeのパス(m l h v q c と大文字・z)を、折れ線に直した多角形のリストにして返す(読める向き、mm)。
    グループの translate / matrix も反映する。fill-rule は偶奇(穴は重なりの偶奇)で扱う"""
    import xml.etree.ElementTree as ET
    root = ET.parse(svgfile).getroot(); NS = '{http://www.w3.org/2000/svg}'; polys = []
    def flat_path(d, m):
        tok = re.findall(r'[MmLlHhVvCcQqZz]|-?\d*\.?\d+(?:e-?\\d+)?', d); i = 0; cur = None; p = (0.0, 0.0); start = (0.0, 0.0); cmd = None
        subs = []
        def num(): 
            nonlocal i; v = float(tok[i]); i += 1; return v
        while i < len(tok):
            if re.match(r'[A-Za-z]', tok[i]): cmd = tok[i]; i += 1
            if cmd in 'Zz':
                if cur: subs.append(cur); cur = None
                p = start; continue
            rel = cmd.islower(); C = cmd.upper()
            if C == 'M':
                x, y = num(), num(); p = (p[0]+x, p[1]+y) if rel else (x, y); start = p
                if cur: subs.append(cur)
                cur = [p]; cmd = 'l' if rel else 'L'
            elif C == 'L':
                x, y = num(), num(); p = (p[0]+x, p[1]+y) if rel else (x, y); cur.append(p)
            elif C == 'H':
                x = num(); p = (p[0]+x if rel else x, p[1]); cur.append(p)
            elif C == 'V':
                y = num(); p = (p[0], p[1]+y if rel else y); cur.append(p)
            elif C == 'Q':
                a = (num(), num()); b = (num(), num())
                if rel: a = (p[0]+a[0], p[1]+a[1]); b = (p[0]+b[0], p[1]+b[1])
                for k in range(1, curve_steps+1):
                    u = k/curve_steps; cur.append(((1-u)**2*p[0] + 2*(1-u)*u*a[0] + u*u*b[0], (1-u)**2*p[1] + 2*(1-u)*u*a[1] + u*u*b[1]))
                p = b
            elif C == 'C':
                a = (num(), num()); b = (num(), num()); c = (num(), num())
                if rel: a = (p[0]+a[0], p[1]+a[1]); b = (p[0]+b[0], p[1]+b[1]); c = (p[0]+c[0], p[1]+c[1])
                for k in range(1, curve_steps+1):
                    u = k/curve_steps; cur.append(tuple((1-u)**3*p[j] + 3*(1-u)**2*u*a[j] + 3*(1-u)*u*u*b[j] + u**3*c[j] for j in (0, 1)))
                p = c
        if cur: subs.append(cur)
        return [[_ap(m, q) for q in sp] for sp in subs if len(sp) >= 3]
    def walk(e, m):
        m = _mul(m, _mat(e.get('transform')))
        if e.tag == NS + 'path' and e.get('d'): polys.extend(flat_path(e.get('d'), m))
        for c in e: walk(c, m)
    walk(root, (1, 0, 0, 1, 0, 0))
    return polys

# ================= 部品(局所座標: 外形の左上が原点)と、シートへの配置(回転・移動) =================
def tf_d(d, f):
    """絶対座標のパス(M L C A Z)の座標に、関数 f(x,y)->(x',y') を適用する。回転・平行移動のみ(円弧の半径・向きは変わらない)"""
    tok = re.findall(r'[MLCAZ]|-?\d*\.?\d+(?:e-?\\d+)?', d); out = []; i = 0
    while i < len(tok):
        c = tok[i]; i += 1
        if c == 'Z': out.append('Z')
        elif c in 'ML' or c == 'C':
            n = 1 if c in 'ML' else 3; pts = []
            for _ in range(n):
                x, y = f(float(tok[i]), float(tok[i+1])); i += 2; pts.append('%s,%s' % (fmt(x), fmt(y)))
            out.append(c + ' ' + ' '.join(pts))
            # 同じコマンドの続き(M/L/C の暗黙の繰り返し)
            while i < len(tok) and not re.match(r'[A-Za-z]', tok[i]):
                pts = []
                for _ in range(n):
                    x, y = f(float(tok[i]), float(tok[i+1])); i += 2; pts.append('%s,%s' % (fmt(x), fmt(y)))
                out.append(('L' if c == 'M' else c) + ' ' + ' '.join(pts))
        elif c == 'A':
            rx, ry, rot, la, sw = tok[i:i+5]; x, y = f(float(tok[i+5]), float(tok[i+6])); i += 7
            out.append('A %s %s %s %s %s %s,%s' % (rx, ry, rot, la, sw, fmt(x), fmt(y)))
    return ' '.join(out)

def part_plate(art_sub, boot_d=3.0, notch=3.0):
    cut = [('outline', 'path', plate_outline_d(0, 0, notch))]
    for k, (hx, hy) in enumerate(HOLES_COVER + HOLES_TOP): cut.append(('hole%d' % (k+1), 'circle', (hx, hy, HOLE_SCREW_D/2)))
    for k, (hx, hy) in enumerate(BOOTRESET): cut.append(('bootreset%d' % (k+1), 'circle', (hx, hy, boot_d/2)))
    eng = [('art%d' % (k+1), d) for k, d in enumerate(art_paths(art_sub, 0, 0))]
    return dict(w=BOARD_W, h=BOARD_H, cut=cut, eng=eng)
def part_cover(xr, slot, text_polys):
    w, h = cover_size(xr)
    cut = [('outline', 'path', cover_outline_d(0, 0, xr, slot))]
    for k, (hx, hy) in enumerate(HOLES_COVER):
        if slot and k == 2: continue                      # 溝にした穴は円を作らない(外形に含まれる)
        cut.append(('hole%d' % (k+1), 'circle', (xr - hx, hy - COVER_BOX[1], HOLE_SCREW_D/2)))
    t = place(text_polys, COVER_TEXT['scale'], *COVER_TEXT['center'], 'up90')
    frost = cover_frost_polys(xr, t, [])
    eng = [('frost', pd([[(xr - px, py - COVER_BOX[1]) for px, py in q] for q in frost]))]
    return dict(w=w, h=h, cut=cut, eng=eng)

def build_layout(pcb, art, logo, layout_json):
    import json
    parts = json.load(open(layout_json, encoding='utf-8'))
    W = H = 300
    art_sub = load_art(art)[0]; text_polys = load_logo_svg(logo)
    P = part_plate(art_sub, boot_d=3.0, notch=3.0)
    CV = {'C1': part_cover(23.75, True, text_polys), 'C2': part_cover(24.5, False, text_polys)}
    cut, eng, n = [], [], {'P': 0, 'C1': 0, 'C2': 0}
    for p in sorted(parts, key=lambda p: (p['kind'] != 'P', p['kind'], p['y'], p['x'])):
        k = p['kind']; n[k] += 1; base = P if k == 'P' else CV[k]
        name = 'P%02d' % n[k] if k == 'P' else '%s_%02d' % (k, n[k])
        rot = p['h'] > p['w'] if k == 'P' else True       # 配置図の向き: ボトムは縦置き(h>w)、カバーは横置き(回転)
        w0, h0 = base['w'], base['h']
        f = (lambda x, y, h0=h0, ox=p['x'], oy=p['y']: (ox + (h0 - y), oy + x)) if rot else (lambda x, y, ox=p['x'], oy=p['y']: (ox + x, oy + y))   # 時計回りに90度
        for nm, kind, d in base['cut']:
            if kind == 'path': cut.append(('%s_%s' % (name, nm), 'path', tf_d(d, f)))
            else:
                x, y = f(d[0], d[1]); cut.append(('%s_%s' % (name, nm), 'circle', (x, y, d[2])))
        for nm, d in base['eng']: eng.append(('%s_%s' % (name, nm), tf_d(d, f)))
    L = ['<?xml version="1.0" encoding="UTF-8"?>',
         '<svg xmlns="http://www.w3.org/2000/svg" xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" width="%dmm" height="%dmm" viewBox="0 0 %d %d">' % (W, H, W, H),
         '<g id="cut" inkscape:groupmode="layer" inkscape:label="cut(赤1pt)" fill="none" stroke="#ff0000" stroke-width="%.4f">' % PT]
    for nm, kind, d in cut:
        L.append('<path id="%s" d="%s"/>' % (nm, d) if kind == 'path' else '<circle id="%s" cx="%s" cy="%s" r="%s"/>' % (nm, fmt(d[0]), fmt(d[1]), fmt(d[2])))
    L.append('</g>')
    L.append('<g id="engrave" inkscape:groupmode="layer" inkscape:label="engrave(黒の塗り)" fill="#000000" stroke="none" fill-rule="evenodd">')
    for nm, d in eng: L.append('<path id="%s" d="%s"/>' % (nm, d))
    L += ['</g>', '</svg>']
    return '\n'.join(L), n

if __name__ == '__main__':
    ap = argparse.ArgumentParser(); ap.add_argument('--pcb', required=True); ap.add_argument('--sheet', default='15x30', choices=['15x30', '30x30']); ap.add_argument('--art', help='ボトムに入れるイラストのSVG(下から見た向き。P?_outline入りの図)'); ap.add_argument('--layout', help='配置のJSON(30×30用の15:15配置)'); ap.add_argument('--logo', help='カバーの抜き文字のSVG(TakoyaKeeb_logo.svg)'); ap.add_argument('--out', required=True)
    a = ap.parse_args()
    if a.layout:
        svg, n = build_layout(a.pcb, a.art, a.logo, a.layout); open(a.out, 'w', encoding='utf-8').write(svg); print('書き出し:', a.out, n); raise SystemExit
    svg, W, H, items = build(a.pcb, a.sheet, a.art)
    open(a.out, 'w', encoding='utf-8').write(svg); print('書き出し:', a.out, '(%dx%dmm, %d部品)' % (W, H, len(items)))

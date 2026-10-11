#!/usr/bin/env python3
"""Anymanyの入稿ルールを、書き出したSVGから自動で検算する(スクリプトの出力そのものを読み直す)。
使い方: python3 check_acrylic_svg.py acrylic_15x30.svg [--render check.png]
必要なパッケージ: pip install numpy pillow scipy matplotlib
"""
import sys, re, math, itertools, argparse
import xml.etree.ElementTree as ET
import numpy as np
from PIL import Image, ImageDraw
from scipy import ndimage as ndi
NS = '{http://www.w3.org/2000/svg}'
PT = 25.4/72

def arc_pts(x1, y1, rx, ry, large, sweep, x2, y2, n=24):
    # 円弧(rx==ry)の端点表現 → 中心を求めて点列に
    r = rx; dx, dy = (x1-x2)/2, (y1-y2)/2; d2 = dx*dx + dy*dy
    if d2 == 0: return []
    r = max(r, math.sqrt(d2)); k = math.sqrt(max(0, (r*r - d2)/d2)) * (-1 if large == sweep else 1)
    cx = k*dy + (x1+x2)/2; cy = -k*dx + (y1+y2)/2
    a1 = math.atan2(y1-cy, x1-cx); a2 = math.atan2(y2-cy, x2-cx); da = a2 - a1
    if sweep and da < 0: da += 2*math.pi
    if not sweep and da > 0: da -= 2*math.pi
    return [(cx + r*math.cos(a1 + da*i/n), cy + r*math.sin(a1 + da*i/n)) for i in range(1, n+1)]
def parse_d(d):
    tok = re.findall(r'[MLACZ]|-?\d*\.?\d+(?:e-?\d+)?', d); i = 0; subs = []; cur = []; p = (0, 0)
    while i < len(tok):
        c = tok[i]; i += 1
        if c == 'M':
            if cur: subs.append(cur)
            p = (float(tok[i]), float(tok[i+1])); i += 2; cur = [p]
        elif c == 'L':
            while i < len(tok) and tok[i] not in 'MLACZ': p = (float(tok[i]), float(tok[i+1])); i += 2; cur.append(p)
        elif c == 'C':
            while i < len(tok) and tok[i] not in 'MLACZ':
                v = [float(t) for t in tok[i:i+6]]; i += 6; p0 = p
                for k in range(1, 17):
                    u = k/16; q = (1-u)**3*np.array(p0) + 3*(1-u)**2*u*np.array(v[0:2]) + 3*(1-u)*u**2*np.array(v[2:4]) + u**3*np.array(v[4:6]); cur.append((float(q[0]), float(q[1])))
                p = (v[4], v[5])
        elif c == 'A':
            rx, ry, rot, la, sw, x, y = [float(t) for t in tok[i:i+7]]; i += 7
            cur += arc_pts(p[0], p[1], rx, ry, int(la), int(sw), x, y); p = (x, y)
        elif c == 'Z':
            subs.append(cur); cur = []
    if cur: subs.append(cur)
    return subs
def seg_min_dist(pts, polyline):
    """点群 pts(N,2) から 折れ線(閉じた)までの最小距離"""
    P = np.asarray(pts); A = np.asarray(polyline); B = np.roll(A, -1, axis=0); AB = B - A
    L2 = (AB**2).sum(1); L2[L2 == 0] = 1e-12; best = np.inf
    for a, ab, l2 in zip(A, AB, L2):
        t = np.clip(((P - a) @ ab)/l2, 0, 1); q = a + t[:, None]*ab
        best = min(best, np.sqrt(((P - q)**2).sum(1)).min())
    return best
def circ_poly(cx, cy, r, n=96): return [(cx + r*math.cos(2*math.pi*i/n), cy + r*math.sin(2*math.pi*i/n)) for i in range(n)]

def main():
    ap = argparse.ArgumentParser(); ap.add_argument('svg'); ap.add_argument('--render'); a = ap.parse_args()
    root = ET.parse(a.svg).getroot(); rows = []
    def add(item, rule, result, ok): rows.append((item, rule, result, 'OK' if ok else '要確認'))
    # 1 ページ
    w, h, vb = root.get('width'), root.get('height'), root.get('viewBox')
    add('ページのサイズ', '材料サイズ(30×30 / 15×30 / 30×60cm)', '%s × %s (viewBox %s)' % (w, h, vb), (w, h) in (('300mm', '150mm'), ('300mm', '300mm'), ('600mm', '300mm')) and vb == '0 0 %s %s' % (w[:-2], h[:-2]))
    pages = len([e for e in root.iter() if e.tag.endswith('svg')])
    add('ページ数', '1ドキュメントに1ページ', '1ページ', pages == 1)
    # 2 要素と書式
    tags = {}; bad = []
    for e in root.iter():
        t = e.tag.replace(NS, ''); tags[t] = tags.get(t, 0) + 1
        for att in ('transform', 'style', 'clip-path', 'mask', 'filter'):
            if e.get(att) is not None: bad.append((t, att))
    add('使っている要素', '文字・画像・効果なし(パスのみ)', ', '.join('%s×%d' % kv for kv in sorted(tags.items())), not any(t in tags for t in ('text', 'image', 'use', 'clipPath', 'mask', 'filter')) and not bad)
    cutg = [g for g in root.iter(NS+'g') if g.get('id') == 'cut'][0]; engg = [g for g in root.iter(NS+'g') if g.get('id') == 'engrave'][0]
    sw = float(cutg.get('stroke-width'))
    add('カットの線', '赤(R255 G0 B0)、1pt、塗りなし', 'stroke=%s 幅=%.4fmm(1pt=%.4f) fill=%s' % (cutg.get('stroke'), sw, PT, cutg.get('fill')), cutg.get('stroke').lower() == '#ff0000' and abs(sw - PT) < 0.001 and cutg.get('fill') == 'none')
    add('彫刻の塗り', '黒(R0 G0 B0)、ストロークなし', 'fill=%s stroke=%s' % (engg.get('fill'), engg.get('stroke')), engg.get('fill').lower() == '#000000' and engg.get('stroke') == 'none')
    # カット図形
    parts = {}
    for e in cutg:
        i = e.get('id'); name = i.split('_hole')[0].split('_bootreset')[0].split('_outline')[0]
        pr = parts.setdefault(name, dict(outline=None, holes=[]))
        if e.tag == NS+'path': pr['outline'] = parse_d(e.get('d'))[0]
        else: pr['holes'].append((float(e.get('cx')), float(e.get('cy')), float(e.get('r'))))
    W, H = float(w[:-2]), float(h[:-2])
    bb = {}
    for n, pr in parts.items():
        X = [p[0] for p in pr['outline']]; Y = [p[1] for p in pr['outline']]; bb[n] = (min(X), min(Y), max(X), max(Y))
    kinds = {}
    for n_ in bb:
        k_ = 'ボトム' if n_.startswith('P') else n_.split('_')[0]; kinds[k_] = kinds.get(k_, 0) + 1
    add('部品の数', '(配置の確認)', ', '.join('%s×%d' % kv for kv in sorted(kinds.items())) + '(合計 %d)' % len(bb), True)
    marg = min(min(b[0], b[1], W-b[2], H-b[3]) for b in bb.values())
    L_ = min(b[0] for b in bb.values()); R_ = W-max(b[2] for b in bb.values()); T_ = min(b[1] for b in bb.values()); B_ = H-max(b[3] for b in bb.values())
    add('端からの余白', '加工範囲は端から3mm内側まで', '最小 %.3fmm(左 %.2f / 右 %.2f / 上 %.2f / 下 %.2f)' % (marg, L_, R_, T_, B_), marg >= 3 - 1e-6)
    gap = 1e9; gp = None
    for (n1, b1), (n2, b2) in itertools.combinations(bb.items(), 2):
        dx = max(b2[0]-b1[2], b1[0]-b2[2]); dy = max(b2[1]-b1[3], b1[1]-b2[3]); g = max(dx, dy) if (dx > 0 or dy > 0) else -1
        if g < gap: gap, gp = g, (n1, n2)
    add('部品どうしの間隔', '3mm以上', '最小 %.3fmm (%s と %s)' % (gap, *gp), gap >= 3 - 1e-6)
    wall = (1e9, ''); 
    for n, pr in parts.items():
        poly = pr['outline']
        for k, (hx, hy, r) in enumerate(pr['holes']):
            d = seg_min_dist([(hx, hy)], poly) - r
            if d < wall[0]: wall = (d, '%s の穴(%.1f,%.1f) φ%.1f → 外形' % (n, hx, hy, 2*r))
            for (hx2, hy2, r2) in pr['holes'][k+1:]:
                d = math.hypot(hx-hx2, hy-hy2) - r - r2
                if d < wall[0]: wall = (d, '%s の穴どうし' % n)
    add('パーツの細さ(穴のまわり・縁の残り幅)', '1mm以上', '最小 %.3fmm (%s)' % wall, wall[0] >= 1 - 1e-6)
    add('線の重なり', '二重に重ならない', '穴と外形の残り幅が正、部品の間隔が正(線どうしは接しない)。同一idの重複なし', wall[0] > 0 and gap > 0 and len(set(e.get('id') for e in cutg)) == len(list(cutg)))
    # 彫刻(部品ごとに、全パスを重ねて調べる)
    res = 0.02; eng_dist = (1e9, ''); engpolys = []; groups = {}
    for e in engg:
        pid = e.get('id'); subs = parse_d(e.get('d')); engpolys.append((pid, subs))
        key = '_'.join(pid.split('_')[:2]) if pid.startswith('C') else pid.split('_')[0]
        groups.setdefault(key, []).append((pid, subs))
    tot_a = tot_thin = 0; worst = (0, ''); led_cov = {}; bykind = {}
    for key, lst in groups.items():
        bx0, by0, bx1, by1 = bb[key]; Wp, Hp = int((bx1-bx0)/res)+2, int((by1-by0)/res)+2
        m = np.zeros((Hp, Wp), bool)
        for pid, subs in lst:
            mm_ = np.zeros((Hp, Wp), bool)
            for sp in subs:
                im = Image.new('1', (Wp, Hp), 0); ImageDraw.Draw(im).polygon([((x-bx0)/res, (y-by0)/res) for x, y in sp], fill=1); mm_ ^= np.array(im, bool)
            m |= mm_
        r = 0.25/res; core = ndi.distance_transform_edt(m) >= r; op = ndi.distance_transform_edt(~core) <= r
        a_ = m.sum()*res*res; t_ = (m & ~op).sum()*res*res; tot_a += a_; tot_thin += t_
        kk = 'P' if key.startswith('P') else key.split('_')[0]; bykind.setdefault(kk, [0, 0, 0]); bykind[kk][0] += a_; bykind[kk][1] += t_; bykind[kk][2] = max(bykind[kk][2], t_/a_ if a_ else 0)
        if a_ > 0 and t_/a_ > worst[0]: worst = (t_/a_, key)
        if key.startswith('P'):
            # ボトム(99.9×40.05)の局所座標のLED位置を、回転(時計回りに90度)も考慮してシート上の位置に直す
            rot = (bx1-bx0) < (by1-by0); cov = []
            for lx in (33.5, 52.55, 71.6, 90.65):
                cx_, cy_ = (40.05-22.0, lx) if rot else (lx, 22.0)
                cov.append(m[int((cy_-2.5)/res):int((cy_+2.5)/res), int((cx_-2.5)/res):int((cx_+2.5)/res)].mean())
            led_cov[key] = cov
        pr = parts[key]; pts = np.array([p for pid, subs in lst for sp in subs for p in sp])
        if len(pts) > 6000: pts = pts[::max(1, len(pts)//6000)]
        d = seg_min_dist(pts, pr['outline'])
        for (hx, hy, r_) in pr['holes']: d = min(d, seg_min_dist(pts, circ_poly(hx, hy, r_, 48)))
        if d < eng_dist[0]: eng_dist = (d, key)
    lc = [v for key, vs in led_cov.items() for v in vs]
    if lc: add('LEDの上を彫刻が覆う割合(参考)', '(拡散の目安。多いほど光が散る)', '4個のLED(5×5mm)のうち、最小 %.0f%% / 最大 %.0f%%(全ボトムで同じ)' % (100*min(lc), 100*max(lc)), min(lc) > 0.5)
    detail = ' / '.join('%s %.1f%%(部品ごとの最大 %.1f%%)' % ('ボトム' if k == 'P' else k, 100*v[1]/v[0], 100*v[2]) for k, v in sorted(bykind.items()))
    add('彫刻の細さ', '0.5mm以上(0.5mm未満の部分の面積の割合。角の先端も数えるので、1割未満なら実質問題なしと見る)', '彫刻の合計 %.0fmm² のうち 0.5mm未満 %.1fmm²(%.1f%%)。種類ごと: %s' % (tot_a, tot_thin, 100*tot_thin/tot_a, detail), worst[0] < 0.10)
    add('彫刻とカット線の最小距離(参考)', '(規定なし。近すぎると縁が欠ける)', '最小 %.3fmm (%s)' % eng_dist, eng_dist[0] > 0.3)
    print('| 項目 | ルール | 結果 | 判定 |\n|---|---|---|---|')
    for r in rows: print('| %s | %s | %s | %s |' % r)
    if a.render:
        import matplotlib; matplotlib.use('Agg'); import matplotlib.pyplot as plt
        from matplotlib.patches import Polygon, Circle
        fig, ax = plt.subplots(figsize=(11, 5.6), dpi=90)
        for pid, subs in engpolys:
            from matplotlib.path import Path as P_; from matplotlib.patches import PathPatch
            v = []; c = []
            for s in subs: v += s + [s[0]]; c += [P_.MOVETO] + [P_.LINETO]*(len(s)-1) + [P_.CLOSEPOLY]
            ax.add_patch(PathPatch(P_(v, c), fc='#444', ec='none'))   # 偶奇ではなく向きに依存するため、見た目は目安
        for n, pr in parts.items():
            ax.add_patch(Polygon(pr['outline'], fc='none', ec='red', lw=0.8))
            for (hx, hy, r_) in pr['holes']: ax.add_patch(Circle((hx, hy), r_, fc='none', ec='red', lw=0.6))
        ax.set_xlim(0, W); ax.set_ylim(H, 0); ax.set_aspect('equal'); fig.savefig(a.render, bbox_inches='tight')
if __name__ == '__main__': main()

"""Generates the RCP For All Camera logo (see docs/brand/logo.md)."""
import math, os
# Usage: python docs/brand/gen_logo.py  (from the repo root)
# Writes internal/gui/brand/{logo,mark-dark,mark-light}.svg and
# docs/brand/logo-construction.svg.
OUT_APP, OUT_DOC = "internal/gui/brand", "docs/brand"
C = 32.0
R = 21.0      # iris (blade) radius
RING = 26.0   # ring radius
RING_W = 2.0
r = 8.0       # opening octagon, vertex radius
G = 1.6       # gap between blades
N = 8
ROT = -90 - 22.5  # first vertex angle (deg), so that a blade sits at 12 o'clock
ORANGE, GREY, TILE, OPEN, RINGC = "#F47B20", "#E8E9EC", "#212225", "#17181A", "#6B6F78"

def pt(a, rad): return (C + rad*math.cos(math.radians(a)), C + rad*math.sin(math.radians(a)))
A = [pt(ROT + 45*k, r) for k in range(N)]

def sub(p,q): return (p[0]-q[0], p[1]-q[1])
def add(p,q): return (p[0]+q[0], p[1]+q[1])
def mul(p,s): return (p[0]*s, p[1]*s)
def norm(p):
    l = math.hypot(*p); return (p[0]/l, p[1]/l)
def cross(a,b): return a[0]*b[1]-a[1]*b[0]

def line_circle(p, d):
    # p + t d on circle radius R around C, t > 0 largest
    f = sub(p, (C,C))
    b = 2*(f[0]*d[0]+f[1]*d[1]); c = f[0]**2+f[1]**2-R*R
    t = (-b + math.sqrt(b*b-4*c))/2
    return add(p, mul(d, t))

def intersect(p1, d1, p2, d2):
    t = cross(sub(p2,p1), d2)/cross(d1,d2)
    return add(p1, mul(d1, t))

def blade(k):
    a0, a1, a2 = A[k], A[(k+1)%N], A[(k+2)%N]
    d1 = norm(sub(a1, a0))   # L_k, ray beyond a1
    d2 = norm(sub(a2, a1))   # L_{k+1}, ray from a1 through a2
    P, Q = line_circle(a1, d1), line_circle(a1, d2)
    mid = pt(math.degrees(math.atan2((P[1]+Q[1])/2-C, (P[0]+Q[0])/2-C)), R)
    # shift each line toward the blade by G/2
    def shifted(p, d):
        n = (-d[1], d[0])
        if (sub(mid, p)[0]*n[0] + sub(mid, p)[1]*n[1]) < 0: n = mul(n, -1)
        return add(p, mul(n, G/2))
    p1, p2 = shifted(a1, d1), shifted(a1, d2)
    V = intersect(p1, d1, p2, d2)
    P, Q = line_circle(V, d1), line_circle(V, d2)
    sweep = 1 if cross(sub(P,(C,C)), sub(Q,(C,C))) > 0 else 0
    ang = math.degrees(math.atan2(mid[1]-C, mid[0]-C))
    f = lambda v: f"{v:.3f}".rstrip('0').rstrip('.')
    d = f"M{f(V[0])} {f(V[1])}L{f(P[0])} {f(P[1])}A{f(R)} {f(R)} 0 0 {sweep} {f(Q[0])} {f(Q[1])}Z"
    return d, ang

blades = [blade(k) for k in range(N)]
top = min(range(N), key=lambda k: abs(((blades[k][1]+90)+180)%360-180))

def mark(tile=True, light=False):
    s = []
    if tile:
        # A path, not <rect rx>: Fyne's SVG renderer ignores rx.
        s.append(f'<path d="M14 0H50A14 14 0 0 1 64 14V50A14 14 0 0 1 50 64H14A14 14 0 0 1 0 50V14A14 14 0 0 1 14 0Z" fill="{TILE}"/>')
    blade_c = "#2B2D31" if light else GREY
    ring_c = "#7A7F89" if light else RINGC
    s.append(f'<circle cx="32" cy="32" r="{RING}" fill="none" stroke="{ring_c}" stroke-width="{RING_W}"/>')
    for k,(d,_) in enumerate(blades):
        s.append(f'<path d="{d}" fill="{ORANGE if k==top else blade_c}"/>')
    return s

def svg(body, title, vb="0 0 64 64", w=64, h=64):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="{vb}">\n'
            f'  <title>{title}</title>\n  ' + "\n  ".join(body) + "\n</svg>\n")

open(os.path.join(OUT_APP, "logo.svg"), "w", encoding="utf-8", newline="\n").write(svg(mark(True), "RCP For All Camera"))
open(os.path.join(OUT_APP, "mark-dark.svg"), "w", encoding="utf-8", newline="\n").write(svg(mark(False), "RCP For All Camera, mark for dark backgrounds"))
open(os.path.join(OUT_APP, "mark-light.svg"), "w", encoding="utf-8", newline="\n").write(svg(mark(False, True), "RCP For All Camera, mark for light backgrounds"))

# construction drawing, 10x scale, with margins for labels
S = 10; M = 120; W = 64*S + 2*M
def X(v): return M + v*S
b = [f'<rect width="{W}" height="{W}" fill="#FFFFFF"/>']
for i in range(0, 65, 2):
    major = i % 16 == 0
    b.append(f'<line x1="{X(i)}" y1="{X(0)}" x2="{X(i)}" y2="{X(64)}" stroke="{"#9AA0AA" if major else "#E3E6EA"}" stroke-width="{1.2 if major else 0.6}"/>')
    b.append(f'<line x1="{X(0)}" y1="{X(i)}" x2="{X(64)}" y2="{X(i)}" stroke="{"#9AA0AA" if major else "#E3E6EA"}" stroke-width="{1.2 if major else 0.6}"/>')
b.append(f'<g transform="translate({M} {M}) scale({S})" opacity="0.28">' + "".join(mark(True)) + '</g>')
cx = X(32)
# guide circles
for rad, col, lab in [(r, "#0A84FF", "r = 8  ouverture (octogone)"), (R, "#0A84FF", "R = 21  lames"), (RING, "#0A84FF", "26  bague (trait 2)"), (32, "#BF5AF2", "tuile 64 × 64, rayon d'angle 14")]:
    b.append(f'<circle cx="{cx}" cy="{cx}" r="{rad*S}" fill="none" stroke="{col}" stroke-width="1.5" stroke-dasharray="6 5"/>')
# octagon and extended sides
pts = " ".join(f"{X(p[0])},{X(p[1])}" for p in A)
b.append(f'<polygon points="{pts}" fill="none" stroke="#FF375F" stroke-width="1.5"/>')
for k in range(N):
    a0, a1 = A[k], A[(k+1)%N]
    d = norm(sub(a1, a0))
    e1, e2 = add(a0, mul(d, -2)), line_circle(a1, d)
    e2 = add(e2, mul(d, 2))
    b.append(f'<line x1="{X(e1[0])}" y1="{X(e1[1])}" x2="{X(e2[0])}" y2="{X(e2[1])}" stroke="#FF375F" stroke-width="1.2" stroke-dasharray="4 4"/>')
for p in A:
    b.append(f'<circle cx="{X(p[0])}" cy="{X(p[1])}" r="4" fill="#FF375F"/>')
b.append(f'<line x1="{cx}" y1="{X(0)}" x2="{cx}" y2="{X(64)}" stroke="#30D158" stroke-width="1.2"/>')
b.append(f'<line x1="{X(0)}" y1="{cx}" x2="{X(64)}" y2="{cx}" stroke="#30D158" stroke-width="1.2"/>')
font = 'font-family="Segoe UI, Arial, sans-serif"'
labels = [
    (X(0), M-70, "Grille 64 × 64 (pas 2, repères tous les 16)", "#3A3F47"),
    (X(0), M-44, "Rouge : octogone d'ouverture r = 8, côtés prolongés jusqu'au cercle R = 21", "#FF375F"),
    (X(0), M-18, "Bleu : cercles de construction · Violet : tuile · Vert : axes", "#0A84FF"),
    (X(0), X(64)+40, "8 lames à 45°, jour entre lames 1,6 · 1 lame orange #F47B20 à 12 h", "#3A3F47"),
    (X(0), X(64)+66, "Lames #E8E9EC · Bague #6B6F78 · Tuile #212225", "#3A3F47"),
]
for x, y, t, c in labels:
    b.append(f'<text x="{x}" y="{y}" {font} font-size="18" fill="{c}">{t}</text>')
open(os.path.join(OUT_DOC, "logo-construction.svg"), "w", encoding="utf-8", newline="\n").write(
    svg(b, "RCP For All Camera, logo construction", vb=f"0 0 {W} {W}", w=W, h=W))


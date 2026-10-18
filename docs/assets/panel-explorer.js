/* Panel explorer: draws the ATEM Micro Camera Panel face (geometry from
   internal/surfaces/panelsim/sim_layout.go) and explains each element when
   it is clicked: its input report, its LEDs and how to light them, and what
   RCP For All Camera does with it. Everything is in drawing units (the
   simulator's), the SVG viewBox does the scaling. */
(function () {
  "use strict";

  var NS = "http://www.w3.org/2000/svg";
  function hex(n) { return (n < 16 ? "0" : "") + n.toString(16).toUpperCase(); }
  function code(n) { return "0x" + hex(n); }

  // --- report builders ---------------------------------------------------------------

  // ledReport returns report 0x02 with the given LEDs on, layer 0.
  function ledReport(leds) {
    var b = new Array(30).fill(0);
    leds.forEach(function (n) { b[n >> 3] |= 1 << (n & 7); });
    return "02 " + b.map(hex).join(" ") + " 00";
  }
  function ledWhere(n) {
    return "LED " + n + ": byte " + (n >> 3) + " of the bitmap, mask " + code(1 << (n & 7));
  }
  // Seven-segment glyphs of report 0x10 (bit 7 = a ... bit 1 = g, bit 0 = point).
  var glyph = { "0": 0xFC, "1": 0x60, "2": 0xDA, "3": 0xF2, "4": 0x66, "5": 0xB6, "6": 0xBE, "7": 0xE0,
    "8": 0xFE, "9": 0xF6, "F": 0x8E, "C": 0x9C, "L": 0x1C, "S": 0xB6, "A": 0xEE, "u": 0x38, "t": 0x1E,
    "o": 0x3A, "-": 0x02, " ": 0x00 };
  // digitReport returns report 0x10 with text written from digit first.
  function digitReport(first, text) {
    var d = new Array(14).fill(0), i = first;
    for (var k = 0; k < text.length; k++) {
      var ch = text[k];
      if (ch === ".") { if (i > first) d[i - 1] |= 1; continue; }
      d[i++] = glyph[ch] || 0;
    }
    return "10 " + d.map(hex).join(" ");
  }

  // --- the elements ------------------------------------------------------------------

  var els = [];
  function add(e) { e.id = "e" + els.length; els.push(e); return e; }

  var keyInfo = function (k, extra) {
    return "<p>Report <code>0x03</code> (input) lists up to six held key codes as u16 little endian; this key is <code>" +
      code(k) + "</code>" + (extra || "") + ". A press shows up as the code appearing, a release as it disappearing.</p>" +
      "<pre>03 " + hex(k) + " 00 00 00 00 00 00 00 00 00 00 00</pre>";
  };
  var ledInfo = function (leds, names) {
    if (!leds.length) return "<p>No LED.</p>";
    var rows = leds.map(function (n, i) { return "<tr><td>" + names[i] + "</td><td><code>" + n + "</code></td><td>byte " + (n >> 3) + ", mask <code>" + code(1 << (n & 7)) + "</code></td></tr>"; }).join("");
    return '<div class="table-wrap"><table><thead><tr><th>LED</th><th>Index</th><th>In the bitmap</th></tr></thead><tbody>' + rows + "</tbody></table></div>" +
      "<p>Send output report <code>0x02</code>: a 30-byte bitmap (LED <em>n</em> is bit <code>n &amp; 7</code> of byte <code>n &gt;&gt; 3</code>) followed by the layer, <code>0</code>. To light " + names[0].toLowerCase() + " alone:</p>" +
      "<pre>" + ledReport([leds[0]]) + "</pre><p>The report sets every LED at once: keep the bits of the other LEDs you want lit.</p>";
  };

  var BTN = [
    { k: 0x4D, x: 366, y: 45, w: 22, h: 11, label: "STORE", t: "STORE", leds: [46, 47], ln: ["White backlight", "Red backlight"], app: "Arms or disarms Store (shared with the window's Store button). Red while armed; the white LED is never used." },
    { k: 0x4E, x: 238, y: 70, w: 16, h: 11, arrow: 1, t: "ND ▲", leds: [48], ln: ["Arrow"], app: "Next ND filter. Lit while the ND can still go up." },
    { k: 0x4F, x: 238, y: 84, w: 16, h: 11, arrow: -1, t: "ND ▼", leds: [49], ln: ["Arrow"], app: "Previous ND filter. Lit while the ND can still go down." },
    { k: 0x50, x: 302, y: 70, w: 16, h: 11, arrow: 1, t: "CC ▲", leds: [50], ln: ["Arrow"], app: "Not mapped yet (no standard colour-correction control)." },
    { k: 0x51, x: 302, y: 84, w: 16, h: 11, arrow: -1, t: "CC ▼", leds: [51], ln: ["Arrow"], app: "Not mapped yet." },
    { k: 0x52, x: 368, y: 70, w: 16, h: 11, arrow: 1, t: "MASTER GAIN ▲", leds: [52], ln: ["Arrow"], app: "Gain up by its step (else the next ISO). In white balance mode: tint up." },
    { k: 0x53, x: 368, y: 84, w: 16, h: 11, arrow: -1, t: "MASTER GAIN ▼", leds: [53], ln: ["Arrow"], app: "Gain down by its step (else the previous ISO). In white balance mode: tint down." },
    { k: 0x54, x: 368, y: 104, w: 16, h: 11, arrow: 1, t: "SHUTTER ▲", leds: [54], ln: ["Arrow"], app: "Next shutter speed (else the next shutter angle). In white balance mode: Kelvin up." },
    { k: 0x55, x: 368, y: 118, w: 16, h: 11, arrow: -1, t: "SHUTTER ▼", leds: [55], ln: ["Arrow"], app: "Previous shutter speed or angle. In white balance mode: Kelvin down." },
    { k: 0x56, x: 238, y: 124, w: 20, h: 11, label: "ON", t: "ON", leds: [57], ln: ["Backlight"], app: "Not mapped yet." },
    { k: 0x57, x: 262, y: 124, w: 20, h: 11, label: "W/B", t: "W/B", leds: [59], ln: ["Backlight"], app: "Short press: white balance mode (GAIN sets the tint, SHUTTER the Kelvin). Held 1 s: auto white balance. Lit in white balance mode." },
    { k: 0x58, x: 286, y: 124, w: 20, h: 11, label: "BARS", t: "BARS", leds: [61], ln: ["Backlight"], app: "Colour bars on or off, only when the current value is known; asks for confirmation while the operator lock is on. Lit while the bars are on." },
    { k: 0x59, x: 262, y: 217, w: 12, h: 10, t: "BLACK/FLARE", leds: [], ln: [], app: "Not mapped. No code was seen in captures: the name of the code is inferred.", note: " (inferred, never seen in a capture)" },
    { k: 0x5A, x: 368, y: 322, w: 20, h: 15, label: "IRIS/MB\nACTIVE", t: "IRIS/MB ACTIVE", leds: [64, 65], ln: ["White backlight", "Red backlight"], app: "The key is not mapped. Red: lever and ring both drive the camera. White: one of them waits for pickup (soft takeover).", note: " (name inferred)" },
    { k: 0x5B, x: 368, y: 346, w: 20, h: 15, label: "AUTO\nIRIS", t: "AUTO IRIS", leds: [66], ln: ["Backlight"], app: "Runs the camera's auto iris. Lit while held.", note: " (name inferred)" },
    { k: 0x5C, x: 368, y: 370, w: 20, h: 13, label: "CALL", t: "CALL", leds: [67, 68], ln: ["White backlight", "Red backlight"], app: "Not mapped.", note: " (name inferred)" },
    { k: 0x5D, x: 238, y: 370, w: 20, h: 15, label: "PANEL\nACTIVE", t: "PANEL ACTIVE", leds: [72, 73], ln: ["White backlight", "Red backlight"], app: "Turns panel control off and on. Red while the panel drives the cameras, white when turned off.", note: " (name inferred)" },
    { k: 0x5E, x: 290, y: 370, w: 20, h: 13, label: "PREVIEW", t: "PREVIEW", leds: [74], ln: ["Backlight"], app: "Not mapped.", note: " (name inferred)" }
  ];
  for (var i = 0; i < 5; i++) {
    BTN.push({ k: 0x48 + i, x: 236 + 25 * i, y: 45, w: 18, h: 11, label: String(i + 1), t: "SCENE " + (i + 1), leds: [41 + i], ln: ["Backlight"],
      app: "Recalls preset " + (i + 1) + " of the active camera, or stores it while Store is armed. Lit when the preset is stored; all five blink while Store is armed." });
  }
  BTN.forEach(function (b) {
    add({ kind: "key", shape: "btn", b: b, title: b.t, sub: "Button",
      html: "<h4>Input</h4>" + keyInfo(b.k, b.note) + "<h4>Light</h4>" + ledInfo(b.leds, b.ln) + "<h4>In RCP For All Camera</h4><p>" + b.app + "</p>" });
  });

  var camTops = [116, 146, 176, 206, 256, 286, 316, 346];
  camTops.forEach(function (y, c) {
    var base = 16 + 3 * c, n = c + 1;
    add({ kind: "cam", shape: "cam", x: 180, y: y, n: n, title: "CAMERA " + n, sub: "Camera button with an RGB backlight",
      html: "<h4>Input</h4>" + keyInfo(0x40 + c) +
        "<h4>Light</h4><p>Three LEDs, one per colour channel: red <code>" + base + "</code>, green <code>" + (base + 1) + "</code>, blue <code>" + (base + 2) +
        "</code>. They are on or off, so the button shows 7 colours: red, green, blue, amber (R+G), cyan (G+B), magenta (R+B), white.</p>" +
        '<div class="table-wrap"><table><thead><tr><th>Colour</th><th>Report 0x02</th></tr></thead><tbody>' +
        [["Red", [base]], ["Green", [base + 1]], ["Blue", [base + 2]], ["Amber", [base, base + 1]]].map(function (r) {
          return "<tr><td>" + r[0] + "</td><td><code>" + ledReport(r[1]) + "</code></td></tr>";
        }).join("") + "</tbody></table></div>" +
        "<h4>In RCP For All Camera</h4><p>Selects slot " + n + " if a camera is assigned (an empty slot's button does nothing). Off: empty slot. Blue: assigned. Amber: active. Red: on air (tally wins over amber).</p>" });
  });

  var IND = [
    { led: 40, x: 231, y: 31, label: "SCENE FILE", app: "Off." },
    { led: 56, x: 231, y: 101, label: "REL", app: "Off." },
    { led: 58, x: 253, y: 101, label: "ABS", app: "Off." },
    { led: 60, x: 277, y: 101, label: "BARS", app: "Lit while the active camera shows colour bars." },
    { led: 62, x: 362, y: 190, label: "D EXT", app: "Off." },
    { led: 63, x: 362, y: 202, label: "EXT", app: "Off." },
    { led: 69, x: 254, y: 362, label: "NETWORK", app: "Lit while the active camera is connected." },
    { led: 70, x: 254, y: 370, label: "ALARM", red: true, app: "Blinks while the active camera is in error, and for 3 s after a command failed." },
    { led: 71, x: 254, y: 378, label: "CABLE", app: "Off." }
  ];
  IND.forEach(function (d) {
    add({ kind: "ind", shape: "ind", d: d, title: d.label + " indicator", sub: "Status light (no key)",
      html: "<h4>Input</h4><p>None: it is a light only.</p><h4>Light</h4>" + ledInfo([d.led], ["The indicator"]) + "<h4>In RCP For All Camera</h4><p>" + d.app + "</p>" });
  });

  var DIG = [
    { first: 0, n: 1, cx: 258, cy: 77, label: "ND", ex: "2", app: "The ND filter as its option number: 0 = clear, 1 = first filter..." },
    { first: 1, n: 1, cx: 280, cy: 77, label: "CC", ex: "", app: "Blank: no standard colour-correction control yet. ATEM Software Control never writes it either." },
    { first: 2, n: 2, cx: 337, cy: 77, label: "MASTER GAIN", ex: "12", app: "Gain in dB (<code>06</code>, <code>-3</code>), else ISO in hundreds (<code>16</code> = 1600). In white balance mode: the tint." },
    { first: 4, n: 4, cx: 330, cy: 111, label: "SHUTTER", ex: "Auto", app: "Shutter speed without 1/ (<code>50</code>), else the angle (<code>180</code>). In white balance mode: Kelvin (<code>5600</code>). While COARSE turns: the iris window centre (<code>F5.6</code>), blinking." },
    { first: 8, n: 3, cx: 251, cy: 297, label: "IRIS", ex: "F2.8", app: "The iris as an f-stop (<code>F2.8</code>, <code>F11</code>)." },
    { first: 11, n: 3, cx: 251, cy: 327, label: "MASTER BLACK", ex: "-5", app: "Master black (<code>12</code>, <code>-5</code>); only negatives carry a sign." }
  ];
  DIG.forEach(function (g) {
    var idx = []; for (var k = 0; k < g.n; k++) idx.push(g.first + k);
    add({ kind: "dig", shape: "dig", g: g, title: g.label + " display", sub: g.n + "-digit seven-segment display",
      html: "<h4>Output</h4><p>Report <code>0x10</code> carries all 14 digits, one byte each, after the report id. This display is digit" + (g.n > 1 ? "s " : " ") +
        "<code>" + idx.join(", ") + "</code>. Each byte is raw segments: bit 7 = a, bit 6 = b ... bit 1 = g, bit 0 = decimal point.</p>" +
        (g.ex ? "<p>To show <code>" + g.ex + "</code> here:</p><pre>" + digitReport(g.first, g.ex) + "</pre>" : "") +
        "<p>Glyphs: <code>0</code>=FC <code>1</code>=60 <code>2</code>=DA <code>3</code>=F2 <code>4</code>=66 <code>5</code>=B6 <code>6</code>=BE <code>7</code>=E0 <code>8</code>=FE <code>9</code>=F6 <code>F</code>=8E <code>-</code>=02, point = <code>|01</code>.</p>" +
        "<h4>In RCP For All Camera</h4><p>" + g.app + " Blank when the camera has no such control, dashes when its value is unknown.</p>" });
  });

  var KNOB = [
    { i: 0, x: 241, y: 160, r: 11, ring: "#e5484d", t: "WHITE R", app: "Red gain, relative: a full turn of the counter spans 0.5 to 1.5." },
    { i: 2, x: 283, y: 160, r: 11, ring: "#46a758", t: "WHITE G", app: "Green gain, relative (0.5 to 1.5)." },
    { i: 4, x: 325, y: 160, r: 11, ring: "#3e63dd", t: "WHITE B", app: "Blue gain, relative (0.5 to 1.5)." },
    { i: 1, x: 241, y: 198, r: 11, ring: "#e5484d", t: "BLACK R", app: "Red lift, relative: a full turn spans -0.5 to +0.5." },
    { i: 3, x: 283, y: 198, r: 11, ring: "#46a758", t: "BLACK G", app: "Green lift, relative (-0.5 to +0.5)." },
    { i: 5, x: 325, y: 198, r: 11, ring: "#3e63dd", t: "BLACK B", app: "Blue lift, relative (-0.5 to +0.5)." },
    { i: 6, x: 368, y: 160, r: 9, t: "Top-right knob", app: "Not mapped (no label on the panel)." },
    { i: 7, x: 368, y: 251, r: 7, label: "SENS", t: "SENS", app: "Width of the iris lever window, from 10 % of the lens' range to all of it. Never writes to the camera." },
    { i: 8, x: 368, y: 279, r: 7, label: "COARSE", t: "COARSE", app: "Centre of the iris lever window. Never writes to the camera; SHUTTER previews the centre while it turns." }
  ];
  KNOB.forEach(function (k) {
    add({ kind: "knob", shape: "knob", k: k, title: k.t + " knob", sub: "Endless rotary knob",
      html: "<h4>Input</h4><p>Report <code>0x0E</code> (input, sent on change) holds the 9 knobs as u16 little endian, absolute 0 to 1024: the panel counts the turns itself and stops at both ends. This knob is index <code>" + k.i +
        "</code>, bytes <code>" + (1 + 2 * k.i) + "–" + (2 + 2 * k.i) + "</code>.</p>" +
        "<p>The same layout is a feature report: read it at connect to get the positions (input reports only come on change). Writing it should move the counters (the app recentres the colour knobs to 512 near an end; not confirmed on hardware).</p>" +
        "<h4>Light</h4><p>No LED.</p><h4>In RCP For All Camera</h4><p>" + k.app + "</p>" });
  });

  add({ kind: "lever", shape: "lever", title: "Iris lever and master black ring", sub: "Lever with a rotating ring on its head",
    html: "<h4>Input</h4><p>Report <code>0x0F</code> holds three int32 little endian: <code>[1, lever, ring]</code>. The lever is bytes <code>5–8</code>, from -4096 (pulled back, closed) to +4096 (pushed forward, open). The ring is bytes <code>9–12</code>, about -4096 to +4096. Also readable as a feature report.</p>" +
      "<pre>0F 01 00 00 00 00 10 00 00 00 00 00 00   lever +4096, ring 0</pre>" +
      "<h4>Light</h4><p>None on the lever: the iris scale beside it and the IRIS/MB ACTIVE button show its state.</p>" +
      "<h4>In RCP For All Camera</h4><p>The lever drives the iris over the mounted lens' range (the camera's normalised iris when it offers one), inside the window set by COARSE and SENS. The ring drives master black, with a small dead zone at the centre. Both use soft takeover: after a camera switch they act only once they reach the camera's value.</p>" });

  add({ kind: "matrix", shape: "matrix", title: "Camera number matrix", sub: "7 × 10 dot matrix, red, green or amber",
    html: "<h4>Output</h4><p>Report <code>0x11</code>: 32 bytes after the id, 2 bits per pixel (0 off, 1 red, 2 green, 3 amber). Pixel <code>p = row × 10 + col</code> sits in byte <code>p &gt;&gt; 2</code>, bits <code>2 × (p &amp; 3)</code>, LSB first. Only 70 pixels exist; bytes 18 to 31 stay 0.</p>" +
      "<p>Numbers 1 to 9 use a centred 5-pixel-wide font, 10 to 99 two 4-pixel-wide digits.</p><h4>In RCP For All Camera</h4><p>The active slot number in amber, red while that camera is on air; <code>-</code> when no slot is selected.</p>" });

  add({ kind: "bar", shape: "bar", title: "Iris scale", sub: "16-segment bar graph",
    html: "<h4>Output</h4><p>16 LEDs of report <code>0x02</code>: index <code>0</code> is the top (above 2.0) and <code>15</code> is CLS. The printed stops 2.0, 2.8 ... 16, CLS sit on segments 1, 3 ... 15 (two segments per stop).</p>" +
      "<p>Light from segment 5 (f/4) down to CLS:</p><pre>" + ledReport([5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15]) + "</pre>" +
      "<h4>In RCP For All Camera</h4><p>The camera's iris as a bar from its opening down to CLS, limited to what the lever window can reach. While the lever waits for pickup, the segment under the lever blinks. While SENS turns: the window itself.</p>" });

  // --- drawing ------------------------------------------------------------------

  function node(tag, attrs, parent) {
    var n = document.createElementNS(NS, tag);
    for (var a in attrs) n.setAttribute(a, attrs[a]);
    if (parent) parent.appendChild(n);
    return n;
  }
  function text(parent, x, y, s, size, fill, anchor, weight) {
    var lines = s.split("\n");
    lines.forEach(function (l, i) {
      var t = node("text", { x: x, y: y + (i - (lines.length - 1) / 2) * size * 1.1, "font-size": size, fill: fill || "#c9c9cf",
        "text-anchor": anchor || "middle", "dominant-baseline": "central", "font-family": "Inter, system-ui, sans-serif", "font-weight": weight || 600 }, parent);
      t.textContent = l;
    });
  }

  function draw(svg) {
    node("rect", { x: 152, y: 14, width: 258, height: 390, rx: 14, fill: "#26272b", stroke: "#3a3b40" }, svg);
    text(svg, 168, 30, "ATEM Micro Camera Panel", 4.6, "#8b8b93", "start", 600);
    text(svg, 283, 145, "WHITE", 3.6, "#8b8b93");
    text(svg, 283, 183, "BLACK", 3.6, "#8b8b93");
    els.forEach(function (e) {
      var g = node("g", { "data-el": e.id, tabindex: 0, role: "button", "aria-label": e.title }, svg);
      var tip = node("title", {}, g); tip.textContent = e.title;
      var hl;
      switch (e.shape) {
        case "btn": {
          var b = e.b, x = b.x - b.w / 2, y = b.y - b.h / 2;
          node("rect", { x: x, y: y, width: b.w, height: b.h, rx: 2, fill: b.k === 0x59 ? "#18181b" : "#d4d4d8" }, g);
          if (b.arrow) node("path", { d: b.arrow > 0 ? "M" + (b.x - 3) + " " + (b.y + 1.5) + "L" + b.x + " " + (b.y - 2) + "L" + (b.x + 3) + " " + (b.y + 1.5) + "Z" : "M" + (b.x - 3) + " " + (b.y - 1.5) + "L" + b.x + " " + (b.y + 2) + "L" + (b.x + 3) + " " + (b.y - 1.5) + "Z", fill: "#3f3f46" }, g);
          if (b.label) text(g, b.x, b.y, b.label, b.label.length > 5 ? 2.6 : 3.4, "#27272a");
          hl = node("rect", { class: "pn-hl", x: x - 1.5, y: y - 1.5, width: b.w + 3, height: b.h + 3, rx: 3 }, g);
          break;
        }
        case "cam":
          node("rect", { x: e.x, y: e.y, width: 30, height: 24, rx: 2.5, fill: "#3a3b40" }, g);
          text(g, e.x + 15, e.y + 12, String(e.n), 8, "#e4e4e7");
          hl = node("rect", { class: "pn-hl", x: e.x - 1.5, y: e.y - 1.5, width: 33, height: 27, rx: 3.5 }, g);
          break;
        case "ind":
          node("circle", { cx: e.d.x, cy: e.d.y, r: 1.6, fill: e.d.red ? "#7f1d1d" : "#52525b" }, g);
          text(g, e.d.x + 3.5, e.d.y, e.d.label, 3.2, "#a1a1aa", "start");
          hl = node("rect", { class: "pn-hl", x: e.d.x - 3, y: e.d.y - 3.5, width: 6 + e.d.label.length * 2.4, height: 7, rx: 2 }, g);
          break;
        case "dig": {
          var gg = e.g, w = gg.n * 7 + (gg.n - 1) * 3, x0 = gg.cx - w / 2;
          node("rect", { x: x0 - 2, y: gg.cy - 8.5, width: w + 4, height: 17, rx: 1.5, fill: "#0b0b0d" }, g);
          for (var k = 0; k < gg.n; k++) text(g, x0 + 3.5 + k * 10, gg.cy, "8", 13, "#3b1f1f", "middle", 500);
          text(g, gg.cx, gg.cy - 12.5, gg.label, 3.2, "#a1a1aa");
          hl = node("rect", { class: "pn-hl", x: x0 - 3.5, y: gg.cy - 16, width: w + 7, height: 26, rx: 2 }, g);
          break;
        }
        case "knob": {
          var kk = e.k;
          node("circle", { cx: kk.x, cy: kk.y, r: kk.r, fill: "#18181b", stroke: kk.ring || "#52525b", "stroke-width": 1.6 }, g);
          node("line", { x1: kk.x, y1: kk.y - kk.r + 2, x2: kk.x, y2: kk.y - kk.r / 2, stroke: "#d4d4d8", "stroke-width": 1 }, g);
          if (kk.label) text(g, kk.x, kk.y + kk.r + 4, kk.label, 3.2, "#a1a1aa");
          hl = node("circle", { class: "pn-hl", cx: kk.x, cy: kk.y, r: kk.r + 2 }, g);
          break;
        }
        case "lever":
          node("rect", { x: 309, y: 244, width: 34, height: 130, rx: 6, fill: "#1c1c1f" }, g);
          node("rect", { x: 323, y: 262, width: 6, height: 94, rx: 3, fill: "#000" }, g);
          node("circle", { cx: 326, cy: 262, r: 15, fill: "#3f3f46", stroke: "#71717a", "stroke-width": 3 }, g);
          node("line", { x1: 326, y1: 251, x2: 326, y2: 256, stroke: "#f4f4f5", "stroke-width": 1.2 }, g);
          text(g, 326, 382, "IRIS / MASTER BLACK", 3, "#a1a1aa");
          hl = node("rect", { class: "pn-hl", x: 307, y: 242, width: 38, height: 134, rx: 7 }, g);
          break;
        case "matrix":
          node("rect", { x: 227, y: 243, width: 48, height: 34.5, rx: 1, fill: "#000" }, g);
          for (var r = 0; r < 7; r++) for (var c = 0; c < 10; c++) node("circle", { cx: 230 + c * 4.5 + 1.2, cy: 246 + r * 4.5 + 1.2, r: 1.5, fill: "#2a1a10" }, g);
          text(g, 251, 238.5, "CAMERA", 3.2, "#a1a1aa");
          hl = node("rect", { class: "pn-hl", x: 225, y: 234, width: 52, height: 46, rx: 2 }, g);
          break;
        case "bar": {
          var labels = { 1: "2.0", 3: "2.8", 5: "4.0", 7: "5.6", 9: "8.0", 11: "11", 13: "16", 15: "CLS" };
          for (var s = 0; s < 16; s++) {
            node("rect", { x: 291, y: 254 + s * 5.8, width: 6, height: 4.2, rx: .5, fill: "#3f2a1a" }, g);
            if (labels[s]) text(g, 288, 254 + s * 5.8 + 2.1, labels[s], 2.8, "#a1a1aa", "end", 500);
          }
          hl = node("rect", { class: "pn-hl", x: 277, y: 251, width: 23, height: 97, rx: 2 }, g);
          break;
        }
      }
      e.g = g;
    });
  }

  function show(e, detail) {
    els.forEach(function (x) { x.g.classList.toggle("sel", x === e); });
    detail.innerHTML = "<h3>" + e.title + '</h3><div class="px-kind">' + e.sub + "</div>" + e.html;
  }

  document.addEventListener("DOMContentLoaded", function () {
    var svg = document.getElementById("pn");
    var detail = document.getElementById("px-detail");
    if (!svg || !detail) return;
    draw(svg);
    svg.addEventListener("click", function (ev) {
      var g = ev.target.closest("[data-el]");
      if (g) show(els[+g.getAttribute("data-el").slice(1)], detail);
    });
    svg.addEventListener("keydown", function (ev) {
      if (ev.key !== "Enter" && ev.key !== " ") return;
      var g = ev.target.closest("[data-el]");
      if (g) { ev.preventDefault(); show(els[+g.getAttribute("data-el").slice(1)], detail); }
    });
    var lever = els.filter(function (x) { return x.kind === "lever"; })[0];
    show(lever, detail);
  });
})();

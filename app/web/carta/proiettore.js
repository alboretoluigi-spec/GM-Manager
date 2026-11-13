/* Proiezione pulita (richiesta dell'utente, 2026-11-12): sul proiettore (o sul secondo schermo) l'immagine va a tutto
   schermo, senza menu, e passa da una regolazione pensata per la proiezione: luminosità, ombre schiarite, contrasto, colori,
   calore e nitidezza. Mappa, immagini, mappe locali, PNG e mostri, video e video dei canti passano tutti da qui.

   - Le regolazioni stanno nel browser del PC del Master (localStorage «gasterbless-proiettore»): la finestra di proiezione
     e la mappa del Master sono nello stesso browser, così ogni cambio nella finestra «📺 Proiezione» si vede subito sul
     proiettore (evento «storage»).
   - Disegno con WebGL: l'immagine (o il fotogramma del video) è ingrandita con un filtro bicubico (Catmull-Rom) quando
     è più piccola dello schermo, poi resa più nitida con un filtro adattivo (come il «contrast adaptive sharpening»: rinforza
     i bordi senza aloni sulle zone già contrastate) e corretta nei colori. Tutto sulla scheda video: anche i video restano
     fluidi. Senza WebGL resta l'elemento originale con un filtro CSS equivalente (senza nitidezza).
   Script classico: espone window.GasterBlessProiettore; lo usa web/carta/carta.js. */
(function () {
  "use strict";
  var CHIAVE = "gasterbless-proiettore", CHIAVE_PROVA = "gasterbless-proiettore-prova";

  // Profili pronti. Valori in punti percentuali, come i cursori.
  var PROFILI = {
    buio: { nome: "Proiettore, stanza buia", luce: 0, ombre: 10, contrasto: 8, colori: 12, calore: 0, nitidezza: 35 },
    luce: { nome: "Proiettore, stanza con luce", luce: 8, ombre: 30, contrasto: 15, colori: 25, calore: 0, nitidezza: 50 },
    led: { nome: "Proiettore LED economico (immagine fredda)", luce: 5, ombre: 20, contrasto: 10, colori: 20, calore: 12, nitidezza: 50 },
    tv: { nome: "Televisore o monitor", luce: 0, ombre: 0, contrasto: 0, colori: 5, calore: 0, nitidezza: 15 },
    neutro: { nome: "Nessuna regolazione", luce: 0, ombre: 0, contrasto: 0, colori: 0, calore: 0, nitidezza: 0 },
  };
  var LIMITI = { luce: [-50, 50], ombre: [0, 60], contrasto: [-30, 40], colori: [-50, 60], calore: [-30, 30], nitidezza: [0, 100], margine: [0, 10] };
  var BASE = { profilo: "buio", adatta: "intera", margine: 0, didascalie: false };
  for (var k in PROFILI.buio) if (k !== "nome") BASE[k] = PROFILI.buio[k];

  function pulisci(o) {
    var r = {}, x = o && typeof o === "object" ? o : {};
    for (var c in LIMITI) {
      var v = Number(x[c]);
      r[c] = isFinite(v) ? Math.round(Math.max(LIMITI[c][0], Math.min(LIMITI[c][1], v))) : BASE[c];
    }
    r.profilo = PROFILI[x.profilo] || x.profilo === "misura" ? x.profilo : BASE.profilo;
    r.adatta = x.adatta === "riempi" ? "riempi" : "intera";
    r.didascalie = x.didascalie === true;
    return r;
  }
  function leggi() { try { return pulisci(JSON.parse(localStorage.getItem(CHIAVE) || "null") || BASE); } catch (e) { return pulisci(BASE); } }
  function salva(r) { var p = pulisci(r); try { localStorage.setItem(CHIAVE, JSON.stringify(p)); } catch (e) { /* niente */ } return p; }
  function conProfilo(r, nome) { var p = PROFILI[nome]; if (!p) return r; var o = pulisci(r); for (var c in p) if (c !== "nome") o[c] = p[c]; o.profilo = nome; return o; }
  function prova() { try { return localStorage.getItem(CHIAVE_PROVA) === "1"; } catch (e) { return false; } }
  function impostaProva(on) { try { if (on) localStorage.setItem(CHIAVE_PROVA, "1"); else localStorage.removeItem(CHIAVE_PROVA); } catch (e) { /* niente */ } }

  // Filtro CSS di riserva (senza WebGL): stessa direzione delle regolazioni, senza nitidezza né calore preciso.
  function filtroCSS(r) {
    var l = 1 + r.luce / 100 + r.ombre / 300, c = 1 + r.contrasto / 100, s = 1 + r.colori / 100;
    return "brightness(" + l.toFixed(3) + ") contrast(" + c.toFixed(3) + ") saturate(" + s.toFixed(3) + ")" + (r.calore > 0 ? " sepia(" + (r.calore / 200).toFixed(3) + ")" : "");
  }

  // ---------- WebGL ----------
  var VS = "attribute vec2 p;varying vec2 v;uniform vec4 uvr;void main(){v=mix(uvr.xy,uvr.zw,p*0.5+0.5);gl_Position=vec4(p,0.0,1.0);}";
  var FS = [
    "#ifdef GL_FRAGMENT_PRECISION_HIGH\nprecision highp float;\n#else\nprecision mediump float;\n#endif\nvarying vec2 v;uniform sampler2D t;uniform vec2 ts;uniform float bic,nit,luce,ombre,contr,sat,cal;",
    // bicubico Catmull-Rom con 9 letture bilineari
    "vec3 bicubico(vec2 uv){vec2 sp=uv*ts;vec2 p1=floor(sp-0.5)+0.5;vec2 f=sp-p1;",
    "vec2 w0=f*(-0.5+f*(1.0-0.5*f));vec2 w1=1.0+f*f*(-2.5+1.5*f);vec2 w2=f*(0.5+f*(2.0-1.5*f));vec2 w3=f*f*(-0.5+0.5*f);",
    "vec2 w12=w1+w2;vec2 o12=w2/w12;vec2 p0=(p1-1.0)/ts;vec2 p3=(p1+2.0)/ts;vec2 p12=(p1+o12)/ts;",
    "vec3 r=texture2D(t,vec2(p0.x,p0.y)).rgb*w0.x*w0.y+texture2D(t,vec2(p12.x,p0.y)).rgb*w12.x*w0.y+texture2D(t,vec2(p3.x,p0.y)).rgb*w3.x*w0.y;",
    "r+=texture2D(t,vec2(p0.x,p12.y)).rgb*w0.x*w12.y+texture2D(t,vec2(p12.x,p12.y)).rgb*w12.x*w12.y+texture2D(t,vec2(p3.x,p12.y)).rgb*w3.x*w12.y;",
    "r+=texture2D(t,vec2(p0.x,p3.y)).rgb*w0.x*w3.y+texture2D(t,vec2(p12.x,p3.y)).rgb*w12.x*w3.y+texture2D(t,vec2(p3.x,p3.y)).rgb*w3.x*w3.y;",
    "return clamp(r,0.0,1.0);}",
    "void main(){vec2 d=1.0/ts;vec3 e=bic>0.5?bicubico(v):texture2D(t,v).rgb;vec3 c=e;",
    // nitidezza adattiva: peso dei vicini secondo il contrasto locale (nessun alone dove è già alto)
    "if(nit>0.0){vec3 n=texture2D(t,v+vec2(0.0,-d.y)).rgb,s=texture2D(t,v+vec2(0.0,d.y)).rgb,w=texture2D(t,v+vec2(-d.x,0.0)).rgb,o=texture2D(t,v+vec2(d.x,0.0)).rgb;",
    "vec3 mn=min(e,min(min(n,s),min(w,o)));vec3 mx=max(e,max(max(n,s),max(w,o)));",
    "vec3 amp=sqrt(clamp(min(mn,1.0-mx)/max(mx,vec3(0.0001)),0.0,1.0));vec3 wg=amp*(-1.0/mix(8.0,4.5,nit));",
    "c=clamp((e+(n+s+w+o)*wg)/(1.0+4.0*wg),0.0,1.0);}",
    // colori: luce (esposizione), ombre (gamma), contrasto attorno al grigio medio, saturazione, calore
    "c*=1.0+luce;c=pow(clamp(c,0.0,1.0),vec3(1.0/(1.0+ombre)));c=(c-0.5)*(1.0+contr)+0.5;",
    "float l=dot(c,vec3(0.2126,0.7152,0.0722));c=mix(vec3(l),c,1.0+sat);",
    "c*=vec3(1.0+cal*0.10,1.0+cal*0.02,1.0-cal*0.12);gl_FragColor=vec4(clamp(c,0.0,1.0),1.0);}",
  ].join("\n");

  function crea(canvas) {
    var gl = null;
    try { gl = canvas.getContext("webgl", { alpha: false, antialias: false, depth: false, premultipliedAlpha: false, powerPreference: "high-performance" }); } catch (e) { gl = null; }
    if (!gl) return null;
    var sh = function (tipo, src) { var s = gl.createShader(tipo); gl.shaderSource(s, src); gl.compileShader(s); return gl.getShaderParameter(s, gl.COMPILE_STATUS) ? s : null; };
    var vs = sh(gl.VERTEX_SHADER, VS), fs = sh(gl.FRAGMENT_SHADER, FS);
    if (!vs || !fs) return null;
    var pr = gl.createProgram(); gl.attachShader(pr, vs); gl.attachShader(pr, fs); gl.linkProgram(pr);
    if (!gl.getProgramParameter(pr, gl.LINK_STATUS)) return null;
    gl.useProgram(pr);
    var b = gl.createBuffer(); gl.bindBuffer(gl.ARRAY_BUFFER, b);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW);
    var ap = gl.getAttribLocation(pr, "p"); gl.enableVertexAttribArray(ap); gl.vertexAttribPointer(ap, 2, gl.FLOAT, false, 0, 0);
    var tex = gl.createTexture(); gl.bindTexture(gl.TEXTURE_2D, tex);
    [gl.TEXTURE_WRAP_S, gl.TEXTURE_WRAP_T].forEach(function (p) { gl.texParameteri(gl.TEXTURE_2D, p, gl.CLAMP_TO_EDGE); });
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR); gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true);
    var U = {}; ["uvr", "ts", "bic", "nit", "luce", "ombre", "contr", "sat", "cal", "t"].forEach(function (n) { U[n] = gl.getUniformLocation(pr, n); });
    gl.uniform1i(U.t, 0);
    var maxT = gl.getParameter(gl.MAX_TEXTURE_SIZE) || 4096;
    return {
      gl: gl,
      maxT: maxT,
      // sorgente: img, video o canvas già pronti; modo: intera (tutta visibile), riempi (copre lo schermo), esatta (stira)
      disegna: function (src, sw, sh2, r, modo, carica) {
        var dpr = Math.min(window.devicePixelRatio || 1, 3);
        var cw = Math.max(1, Math.round(canvas.clientWidth * dpr)), ch = Math.max(1, Math.round(canvas.clientHeight * dpr));
        if (canvas.width !== cw || canvas.height !== ch) { canvas.width = cw; canvas.height = ch; }
        gl.clearColor(0, 0, 0, 1); gl.clear(gl.COLOR_BUFFER_BIT);
        if (!sw || !sh2) return;
        if (carica !== false) gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGB, gl.RGB, gl.UNSIGNED_BYTE, src);
        var sc = modo === "riempi" ? Math.max(cw / sw, ch / sh2) : Math.min(cw / sw, ch / sh2);
        var vw = modo === "esatta" ? cw : Math.min(cw, Math.round(sw * sc)), vh = modo === "esatta" ? ch : Math.min(ch, Math.round(sh2 * sc));
        var u0 = 0, v0 = 0, u1 = 1, v1 = 1;
        if (modo === "riempi") { var fu = cw / (sw * sc), fv = ch / (sh2 * sc); u0 = (1 - fu) / 2; u1 = 1 - u0; v0 = (1 - fv) / 2; v1 = 1 - v0; }
        gl.viewport(Math.round((cw - vw) / 2), Math.round((ch - vh) / 2), vw, vh);
        gl.uniform4f(U.uvr, u0, v0, u1, v1);
        gl.uniform2f(U.ts, sw, sh2);
        gl.uniform1f(U.bic, (modo === "esatta" ? Math.max(cw / sw, ch / sh2) : sc) > 1.15 ? 1 : 0);
        gl.uniform1f(U.nit, r.nitidezza / 100); gl.uniform1f(U.luce, r.luce / 100); gl.uniform1f(U.ombre, r.ombre / 100);
        gl.uniform1f(U.contr, r.contrasto / 100); gl.uniform1f(U.sat, r.colori / 100); gl.uniform1f(U.cal, r.calore / 100);
        gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
      },
    };
  }

  // Un'immagine più grande della texture massima della scheda video si riduce prima con un canvas 2D.
  function adattaMax(src, w, h, maxT) {
    if (w <= maxT && h <= maxT) return { src: src, w: w, h: h };
    var k = maxT / Math.max(w, h), c = document.createElement("canvas");
    c.width = Math.floor(w * k); c.height = Math.floor(h * k);
    c.getContext("2d").drawImage(src, 0, 0, c.width, c.height);
    return { src: c, w: c.width, h: c.height };
  }

  /* mostra(contenitore, sorgente, opz): mette nel contenitore un canvas che disegna la sorgente (img, video o canvas) con
     le regolazioni. opz.modo: "intera" | "riempi" | "esatta" (di base la scelta «adatta» delle regolazioni).
     Restituisce { aggiorna(), ferma(), webgl }. La sorgente resta nel contenitore, invisibile (il video continua a suonare). */
  function mostra(contenitore, src, opz) {
    opz = opz || {};
    var r = leggi(), video = src.tagName === "VIDEO", fermo = false, idCb = 0, rafId = 0;
    var canvas = document.createElement("canvas");
    canvas.className = "pjgl";
    canvas.setAttribute("aria-hidden", "true");
    var R = crea(canvas);
    var modo = function () { return opz.modo || r.adatta; };
    if (!R) { // riserva: l'elemento stesso, a tutto schermo, con il filtro CSS
      src.classList.add("pjriserva");
      var css = function () { src.style.filter = filtroCSS(r); src.style.objectFit = modo() === "riempi" ? "cover" : modo() === "esatta" ? "fill" : "contain"; };
      css();
      return { webgl: false, aggiorna: function () { r = leggi(); css(); }, ferma: function () {} };
    }
    src.classList.add("pjsorgente");
    contenitore.appendChild(canvas);
    var dim = function () { return video ? [src.videoWidth, src.videoHeight] : src.tagName === "IMG" ? [src.naturalWidth, src.naturalHeight] : [src.width, src.height]; };
    var pronto = null;
    var disegna = function (carica) {
      if (fermo) return;
      var d = dim(); if (!d[0] || !d[1]) { R.disegna(null, 0, 0, r, modo()); return; }
      try {
        if (!video) { if (!pronto || carica) pronto = adattaMax(src, d[0], d[1], R.maxT); R.disegna(pronto.src, pronto.w, pronto.h, r, modo(), carica !== false); }
        else R.disegna(src, d[0], d[1], r, modo());
      } catch (e) { /* fotogramma non ancora pronto */ }
    };
    if (video) {
      var ciclo = function () {
        if (fermo) return;
        disegna(true);
        if (src.requestVideoFrameCallback) idCb = src.requestVideoFrameCallback(ciclo); else rafId = requestAnimationFrame(ciclo);
      };
      if (src.readyState >= 2) ciclo(); else src.addEventListener("loadeddata", ciclo, { once: true });
      src.addEventListener("seeked", function () { disegna(true); });
    } else if (src.tagName === "IMG" && !(src.complete && src.naturalWidth)) src.addEventListener("load", function () { disegna(true); }, { once: true });
    else disegna(true);
    var ridim = function () { disegna(video); };
    window.addEventListener("resize", ridim);
    canvas.addEventListener("webglcontextlost", function (e) { e.preventDefault(); }, false);
    canvas.addEventListener("webglcontextrestored", function () { R = crea(canvas) || R; pronto = null; disegna(true); }, false);
    return {
      webgl: true,
      canvas: canvas,
      aggiorna: function (nuova) { r = leggi(); if (nuova) pronto = null; disegna(true); },
      ferma: function () {
        fermo = true; window.removeEventListener("resize", ridim);
        try { var lc = R.gl.getExtension("WEBGL_lose_context"); if (lc) lc.loseContext(); } catch (e) { /* niente */ } // libera subito la scheda video
        if (idCb && src.cancelVideoFrameCallback) src.cancelVideoFrameCallback(idCb);
        if (rafId) cancelAnimationFrame(rafId);
      },
    };
  }

  // ---------- Schermata di prova (per regolare il proiettore) ----------
  function schermataProva(w, h) {
    w = w || 1920; h = h || 1080;
    var c = document.createElement("canvas"); c.width = w; c.height = h;
    var x = c.getContext("2d");
    x.fillStyle = "#000"; x.fillRect(0, 0, w, h);
    x.textBaseline = "middle"; x.textAlign = "center";
    var testo = function (t, px, py, size, col) { x.font = "bold " + size + "px Georgia, serif"; x.fillStyle = col || "#ddd"; x.fillText(t, px, py); };
    testo("Schermata di prova del proiettore", w / 2, h * 0.06, Math.round(h * 0.04), "#d9a441");
    // gradini di grigio 0-100%
    var y = h * 0.11, bh = h * 0.11, n = 11, bw = w * 0.9 / n, x0 = w * 0.05;
    for (var i = 0; i < n; i++) { var g = Math.round(255 * i / (n - 1)); x.fillStyle = "rgb(" + g + "," + g + "," + g + ")"; x.fillRect(x0 + i * bw, y, bw, bh); testo(i * 10 + "%", x0 + i * bw + bw / 2, y + bh + h * 0.02, Math.round(h * 0.018), "#bbb"); }
    // neri vicini: si devono distinguere i primi riquadri (ombre)
    y = h * 0.33; var neri = [0, 2, 4, 6, 8, 10, 12, 15];
    testo("Zone scure: devi distinguere i riquadri dal 4% in su (cursore «Ombre»)", w * 0.27, y - h * 0.03, Math.round(h * 0.018));
    neri.forEach(function (p, j) { var g = Math.round(255 * p / 100); x.fillStyle = "rgb(" + g + "," + g + "," + g + ")"; x.fillRect(w * 0.05 + j * w * 0.055, y, w * 0.05, h * 0.09); testo(p + "%", w * 0.075 + j * w * 0.055, y + h * 0.11, Math.round(h * 0.016), "#999"); });
    // bianchi vicini: non devono fondersi (cursore «Luminosità»)
    var bianchi = [85, 88, 91, 94, 97, 100];
    testo("Zone chiare: non devono diventare tutte uguali (cursore «Luminosità»)", w * 0.75, y - h * 0.03, Math.round(h * 0.018));
    bianchi.forEach(function (p, j) { var g = Math.round(255 * p / 100); x.fillStyle = "rgb(" + g + "," + g + "," + g + ")"; x.fillRect(w * 0.53 + j * w * 0.07, y, w * 0.065, h * 0.09); testo(p + "%", w * 0.5625 + j * w * 0.07, y + h * 0.11, Math.round(h * 0.016), "#999"); });
    // colori: barre pure e tinte tipiche di Gaster Bless (pelle, nebbia, sangue, oro, foresta)
    y = h * 0.53;
    var col = [["#ff0000", "rosso"], ["#00ff00", "verde"], ["#0000ff", "blu"], ["#00ffff", "ciano"], ["#ff00ff", "magenta"], ["#ffff00", "giallo"],
      ["#e0ac8c", "pelle"], ["#8a9a8e", "nebbia"], ["#7a0f17", "sangue"], ["#d9a441", "oro"], ["#2f4a2a", "foresta"]];
    var cw2 = w * 0.9 / col.length;
    col.forEach(function (cc, j) { x.fillStyle = cc[0]; x.fillRect(x0 + j * cw2, y, cw2, h * 0.12); testo(cc[1], x0 + j * cw2 + cw2 / 2, y + h * 0.14, Math.round(h * 0.017), "#bbb"); });
    // nitidezza: righe sottili alternate e un testo piccolo
    y = h * 0.74;
    testo("Nitidezza: le righe devono restare separate, il testo leggibile", w * 0.27, y - h * 0.02, Math.round(h * 0.018));
    for (var k2 = 0; k2 < 3; k2++) {
      var px = w * 0.06 + k2 * w * 0.14, passo = k2 + 1;
      for (var q = 0; q < h * 0.12; q += passo * 2) { x.fillStyle = "#fff"; x.fillRect(px, y + q, w * 0.055, passo); }
      for (var q2 = 0; q2 < w * 0.055; q2 += passo * 2) { x.fillStyle = "#fff"; x.fillRect(px + w * 0.065 + q2, y, passo, h * 0.12); }
    }
    x.textAlign = "left";
    [0.016, 0.022, 0.03, 0.04].forEach(function (s, j) { x.font = Math.round(s * h) + "px Georgia, serif"; x.fillStyle = "#fff"; x.fillText("l'antagonista osserva da Corvonero.", w * 0.56, y + h * 0.01 + j * h * 0.04); });
    x.textAlign = "center";
    // bordi: se il proiettore taglia, aumenta il «Margine ai bordi»
    x.strokeStyle = "#fff"; x.lineWidth = Math.max(2, h / 360); x.strokeRect(x.lineWidth / 2, x.lineWidth / 2, w - x.lineWidth, h - x.lineWidth);
    var ang = h * 0.05;
    [[0, 0, 1, 1], [w, 0, -1, 1], [0, h, 1, -1], [w, h, -1, -1]].forEach(function (a) { x.fillStyle = "#fff"; x.beginPath(); x.moveTo(a[0], a[1]); x.lineTo(a[0] + a[2] * ang, a[1]); x.lineTo(a[0], a[1] + a[3] * ang); x.closePath(); x.fill(); });
    testo("Devi vedere i quattro triangoli bianchi negli angoli e la cornice: altrimenti aumenta il «Margine ai bordi»", w / 2, h * 0.94, Math.round(h * 0.02));
    return c;
  }

  window.GasterBlessProiettore = { CHIAVE: CHIAVE, CHIAVE_PROVA: CHIAVE_PROVA, PROFILI: PROFILI, LIMITI: LIMITI, pulisci: pulisci, leggi: leggi, salva: salva,
    conProfilo: conProfilo, prova: prova, impostaProva: impostaProva, filtroCSS: filtroCSS, mostra: mostra, schermataProva: schermataProva };
})();

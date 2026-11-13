// Testi formattati di Gaster Bless (2026-11-09): riassunti e background.
// - estraiTesto(file): da .txt/.md, Word .docx o PDF a un testo con una formattazione leggera
//   («# titolo», «## sottotitolo», «- voce di elenco», **grassetto**, paragrafi separati da una riga vuota).
// - mostraTesto(testo): lo disegna nello stile di Gaster Bless (titoli, paragrafi, elenchi), senza HTML del file:
//   tutto passa da textContent, quindi un file non può inserire codice nella pagina.
import { el } from "./ui.js";

const MAX_FILE = 15 * 1048576;

function carica(src, nome) {
  return new Promise((ok, ko) => {
    if (document.querySelector('script[src="' + src + '"]')) return ok();
    const s = document.createElement("script");
    s.src = src; s.onload = () => ok(); s.onerror = () => ko(new Error("Impossibile leggere i file " + nome + "."));
    document.head.append(s);
  });
}

// HTML (da Word) → testo formattato: titoli, paragrafi, elenchi, grassetto.
function daHTML(html) {
  const d = new DOMParser().parseFromString(html, "text/html");
  const out = [];
  const inline = (n) => [...n.childNodes].map((c) => c.nodeType === 3 ? c.textContent
    : /^(STRONG|B)$/.test(c.nodeName) && c.textContent.trim() ? "**" + c.textContent.trim() + "**" : inline(c)).join("").replace(/\s+/g, " ").trim();
  for (const n of d.body.children) {
    const t = inline(n);
    if (/^H1$/.test(n.nodeName) && t) out.push("# " + t);
    else if (/^H[2-6]$/.test(n.nodeName) && t) out.push("## " + t);
    else if (/^(UL|OL)$/.test(n.nodeName)) out.push([...n.querySelectorAll("li")].map((li) => "- " + inline(li)).filter((x) => x.length > 2).join("\n"));
    else if (t) out.push(t);
  }
  return out.filter(Boolean).join("\n\n");
}

// PDF → testo: righe ricostruite dalle posizioni, paragrafi dove lo spazio tra le righe cresce, titoli dove il
// carattere è più grande del normale.
async function daPDF(dati) {
  await carica("/vendor/pdfjs/pdf.min.js", "PDF");
  const lib = window.pdfjsLib;
  if (lib && lib.GlobalWorkerOptions && !lib.GlobalWorkerOptions.workerSrc) lib.GlobalWorkerOptions.workerSrc = "/vendor/pdfjs/pdf.worker.min.js";
  const doc = await lib.getDocument({ data: dati, isEvalSupported: false }).promise;
  const righe = [];
  for (let p = 1; p <= doc.numPages; p++) {
    const tc = await (await doc.getPage(p)).getTextContent();
    let cur = null;
    for (const it of tc.items) {
      if (!("str" in it)) continue;
      const y = Math.round(it.transform[5]), h = Math.abs(it.transform[3]) || it.height || 10;
      if (!cur || Math.abs(cur.y - y) > h * 0.5) { if (cur) righe.push(cur); cur = { y, h, t: "", p, x: it.transform[4] }; }
      cur.t += (cur.t && !cur.t.endsWith(" ") && !it.str.startsWith(" ") ? " " : "") + it.str;
      cur.h = Math.max(cur.h, h);
    }
    if (cur) righe.push(cur);
    righe.push({ fine: true });
  }
  const altezze = righe.filter((r) => !r.fine && r.t.trim()).map((r) => r.h).sort((a, b) => a - b);
  const normale = altezze[Math.floor(altezze.length / 2)] || 10;
  // Margine sinistro più frequente: le righe rientrate e consecutive sono un elenco (i punti dell'elenco spesso sono
  // disegnati come figure e non arrivano come testo).
  const xs = {};
  righe.filter((r) => !r.fine && r.t.trim()).forEach((r) => { const k = Math.round(r.x / 4); xs[k] = (xs[k] || 0) + 1; });
  const margine = (+Object.keys(xs).sort((a, b) => xs[b] - xs[a])[0] || 0) * 4;
  const rientrata = (r) => r && !r.fine && r.t.trim() && r.x > margine + normale * 0.9 && r.t.length < 120;
  righe.forEach((r, i) => { if (rientrata(r) && (rientrata(righe[i - 1]) || rientrata(righe[i + 1]))) r.elenco = true; });
  const out = [];
  let par = "", prec = null;
  const chiudi = () => { if (par.trim()) out.push(par.trim()); par = ""; };
  for (const r of righe) {
    if (r.fine) { prec = null; continue; }
    const t = r.t.replace(/\s+/g, " ").trim();
    if (!t) { chiudi(); continue; }
    if (r.h > normale * 1.25 && t.length < 90) { chiudi(); out.push((r.h > normale * 1.6 ? "# " : "## ") + t); prec = r; continue; }
    if (/^[•●▪◦\-–]\s*/.test(t) || r.elenco) { chiudi(); out.push("- " + t.replace(/^[•●▪◦\-–]\s*/, "")); prec = r; continue; }
    if (prec && Math.abs(prec.y - r.y) > r.h * 1.9) chiudi();
    // una parola spezzata a fine riga («castel-» + «lo») si riunisce
    if (par && /[a-zàèéìòù]-$/i.test(par) && /^[a-zàèéìòù]/.test(t)) par = par.slice(0, -1) + t;
    else par += (par ? " " : "") + t;
    prec = r;
  }
  chiudi();
  // le voci di elenco consecutive restano unite (una riga ciascuna)
  return out.join("\n\n").replace(/(^- .*)\n\n(?=- )/gm, "$1\n").replace(/\n{3,}/g, "\n\n");
}

/** Testo formattato da un file (.txt, .md, .docx, .pdf). */
export async function estraiTesto(f) {
  if (f.size > MAX_FILE) throw new Error("File troppo grande: massimo 15 MB.");
  const nome = f.name.toLowerCase();
  if (nome.endsWith(".docx")) {
    await carica("/vendor/mammoth/mammoth.browser.min.js", "Word");
    return daHTML((await window.mammoth.convertToHtml({ arrayBuffer: await f.arrayBuffer() })).value);
  }
  if (nome.endsWith(".pdf") || f.type === "application/pdf") return daPDF(new Uint8Array(await f.arrayBuffer()));
  if (/\.(txt|md|text)$/.test(nome) || f.type.startsWith("text/")) return (await f.text()).replace(/\r\n?/g, "\n").replace(/\n{3,}/g, "\n\n").trim();
  if (nome.endsWith(".doc")) throw new Error("I vecchi file .doc non si leggono: salvalo come .docx o PDF.");
  throw new Error("Formato non supportato: usa un file di testo, Word (.docx) o PDF.");
}
export const ACCETTA_TESTI = ".txt,.md,.docx,.pdf,text/plain,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document";

// **grassetto** dentro una riga
function riga(t) {
  const parti = t.split(/(\*\*[^*]+\*\*)/g).filter(Boolean);
  return parti.map((p) => /^\*\*[^*]+\*\*$/.test(p) ? el("strong", { testo: p.slice(2, -2) }) : document.createTextNode(p));
}

/** Disegna un testo formattato nello stile di Gaster Bless. */
export function mostraTesto(testo) {
  const box = el("div", { class: "testo-gasterbless" });
  const blocchi = String(testo || "").replace(/\r\n?/g, "\n").split(/\n\s*\n/);
  for (const b of blocchi) {
    const righe = b.split("\n").map((x) => x.trimEnd()).filter((x) => x.trim());
    if (!righe.length) continue;
    let ul = null, p = null;
    for (const r of righe) {
      const t = r.trim();
      if (/^#\s+/.test(t)) { ul = p = null; box.append(el("h3", {}, riga(t.replace(/^#\s+/, "")))); continue; }
      if (/^#{2,}\s+/.test(t)) { ul = p = null; box.append(el("h4", {}, riga(t.replace(/^#+\s+/, "")))); continue; }
      if (/^([-*•]|\d+[.)])\s+/.test(t)) {
        p = null;
        if (!ul) { ul = el("ul"); box.append(ul); }
        ul.append(el("li", {}, riga(t.replace(/^([-*•]|\d+[.)])\s+/, ""))));
        continue;
      }
      ul = null;
      if (!p) { p = el("p"); box.append(p); } else p.append(el("br"));
      p.append(...riga(t));
    }
  }
  // Capolettera solo se il primo paragrafo è lungo (su una frase breve spezzerebbe la parola: «S toria»).
  const primo = box.firstElementChild;
  if (primo && primo.tagName === "P" && primo.textContent.length > 160) primo.classList.add("capolettera");
  return box;
}

// Benvenuto (richiesta dell'utente, 2026-11-07): quando un giocatore entra in Gaster Bless parte il video scelto dal Master
// (web/img/benvenuto.mp4), a tutto schermo su sfondo nero, con la scritta «Bentornato in GASTER BLESS».
// Una volta per apertura dell'app (sessionStorage): non riparte a ogni cambio di sezione. Il Master non lo vede.
// Si chiude da solo alla fine del video oppure toccando lo schermo / con Esc.
// Se il browser non permette l'audio senza un tocco (app riaperta dall'icona), il video parte muto e un tocco attiva
// l'audio invece di chiudere.
const CHIAVE = "gasterbless.benvenuto";

export function benvenutoGiaVisto() {
  try { return sessionStorage.getItem(CHIAVE) === "1"; } catch { return false; }
}

export function mostraBenvenuto() {
  try { sessionStorage.setItem(CHIAVE, "1"); } catch { /* navigazione privata: pazienza */ }
  document.querySelector(".benvenuto")?.remove();
  let fatto;
  const chiuso = new Promise((ok) => { fatto = ok; });
  const o = document.createElement("div");
  o.className = "benvenuto";
  o.setAttribute("role", "dialog");
  o.setAttribute("aria-label", "Bentornato in Gaster Bless");
  const v = document.createElement("video");
  v.src = "/img/benvenuto.mp4";
  v.playsInline = true;
  v.setAttribute("playsinline", "");
  v.preload = "auto";
  const t = document.createElement("p");
  t.className = "benvenuto-testo";
  t.append("Bentornato in ", Object.assign(document.createElement("span"), { textContent: "GASTER BLESS" }));
  const nota = document.createElement("span");
  nota.className = "benvenuto-nota";
  nota.textContent = "tocca per chiudere";
  o.append(v, t, nota);
  let muto = false;
  let limite = 0;
  const chiudi = () => {
    if (o.classList.contains("via")) return;
    clearTimeout(limite);
    o.classList.add("via");
    document.removeEventListener("keydown", esc);
    setTimeout(() => { v.pause(); o.remove(); fatto(); }, 600);
  };
  const esc = (e) => { if (e.key === "Escape") chiudi(); };
  o.addEventListener("click", () => {
    if (muto) { muto = false; v.muted = false; nota.textContent = "tocca per chiudere"; return; }
    chiudi();
  });
  document.addEventListener("keydown", esc);
  o.__chiudi = chiudi; // anche per Esc premuto dentro la mappa o la scheda (ui.js)
  v.addEventListener("ended", () => setTimeout(chiudi, 800));
  v.addEventListener("error", () => setTimeout(chiudi, 2500)); // video mancante: resta solo la scritta
  document.body.append(o);
  try { o.setAttribute("tabindex", "-1"); o.focus({ preventScroll: true }); } catch { /* ignora */ } // Esc anche se prima si era toccata la mappa
  v.play().catch(() => {
    muto = true; v.muted = true; nota.textContent = "🔊 tocca per l'audio";
    v.play().catch(() => setTimeout(chiudi, 3000));
  });
  limite = setTimeout(chiudi, 20000); // in ogni caso non resta aperto per sempre
  return chiuso;
}

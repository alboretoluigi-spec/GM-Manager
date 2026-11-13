// Scheda anche senza connessione (2026-11-07). Il dispositivo del giocatore tiene una copia della sua scheda e
// dell'accesso: senza rete l'app si apre sulle schede, la scheda si consulta e si modifica, e le modifiche restano sul
// dispositivo finché il PC del Master non risponde di nuovo. Allora partono con il segno «offline»: al primo accesso in
// sessione il Master viene avvisato (controllo delle schede, internal/server/controllo_schede.go).
// Tutto è in localStorage, una chiave per utente e personaggio; se il ritratto non ci sta, la copia è salvata senza.
import { api } from "./ui.js";
import { appiattisci } from "./campi_scheda.js";

const CHIAVE_IO = "gasterbless.io";
const PREFISSO = "gasterbless.scheda.";

function leggi(k) { try { const s = localStorage.getItem(k); return s ? JSON.parse(s) : null; } catch { return null; } }
function scrivi(k, v) { try { localStorage.setItem(k, JSON.stringify(v)); return true; } catch { return false; } }

// ----- Accesso -----
export function salvaIo(io) {
  if (!io || !io.collegato || !io.utente || io.utente.stato !== "approvato") { try { localStorage.removeItem(CHIAVE_IO); } catch { /* */ } return; }
  const c = { ...io, csrf: "" }; // il token anti-CSRF non serve offline
  scrivi(CHIAVE_IO, c);
}
export function ioSalvato() { return leggi(CHIAVE_IO); }

// ----- Copie delle schede -----
const chiave = (utente, pg) => PREFISSO + utente + "." + pg;

/** Copia di una scheda: { personaggio, dati, modificabile, salvato, base, pendente, stato, offlineDal, quando }. */
export function copia(utente, pg) { return leggi(chiave(utente, pg)); }

export function salvaCopia(utente, pg, c) {
  const v = { ...c, quando: Date.now() };
  if (scrivi(chiave(utente, pg), v)) return true;
  // Spazio esaurito (ritratti grandi): si salva senza ritratto, togliendolo in modo uguale da dati, stato e «salvato»
  // così il ritratto non risulta mai cambiato né cancellato.
  const senza = (o) => (o && typeof o === "object" ? { ...o, img: "" } : o);
  const s = { ...v, dati: senza(v.dati), stato: senza(v.stato), salvato: v.salvato ? { ...v.salvato, img: '""' } : v.salvato, senzaRitratto: true };
  return scrivi(chiave(utente, pg), s);
}

export function copieDi(utente) {
  const out = [];
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i);
      if (k && k.startsWith(PREFISSO + utente + ".")) { const c = leggi(k); if (c && c.personaggio) out.push(c); }
    }
  } catch { /* archivio non disponibile */ }
  return out;
}

export function togliCopia(utente, pg) { try { localStorage.removeItem(chiave(utente, pg)); } catch { /* */ } }

/** Modifiche da inviare: i campi cambiati rispetto all'ultimo stato confermato dal server. */
export function modificheDa(salvato, stato) {
  const nuovo = appiattisci(stato);
  const modifiche = {}, rimossi = [];
  for (const [k, v] of Object.entries(nuovo)) if (salvato[k] !== v) modifiche[k] = JSON.parse(v);
  for (const k of Object.keys(salvato)) if (!(k in nuovo)) rimossi.push(k);
  return { modifiche, rimossi, nuovo };
}

/** Al ritorno della rete: invia le modifiche rimaste sul dispositivo (anche senza aprire la scheda). Se il server
 *  segnala un conflitto, la copia resta: la risolve il giocatore aprendo la scheda. */
export async function sincronizza(utente) {
  let inviate = 0;
  for (const c of copieDi(utente)) {
    if (!c.pendente || !c.stato) continue;
    const { modifiche, rimossi, nuovo } = modificheDa(c.salvato || {}, c.stato);
    if (!Object.keys(modifiche).length && !rimossi.length) { salvaCopia(utente, c.personaggio.id, { ...c, pendente: false, stato: null }); continue; }
    try {
      const r = await api("/personaggi/" + encodeURIComponent(c.personaggio.id) + "/scheda",
        { metodo: "PATCH", dati: { base: c.base, modifiche, rimossi, forza: [], offline: true } });
      salvaCopia(utente, c.personaggio.id, { ...c, dati: c.stato, salvato: nuovo, base: r.versione, pendente: false, stato: null, offlineDal: 0,
        personaggio: { ...c.personaggio, versione: r.versione } });
      inviate++;
    } catch (e) {
      if (e.stato === 0) break;                 // ancora senza rete
      if (e.stato === 403 || e.stato === 404) togliCopia(utente, c.personaggio.id); // scheda non più modificabile
      // 409 (conflitto): resta in sospeso, si risolve aprendo la scheda
    }
  }
  return inviate;
}

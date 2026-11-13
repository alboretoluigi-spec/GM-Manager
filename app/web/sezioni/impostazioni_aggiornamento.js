// Impostazioni → «Aggiornamento di Gaster Bless» (tasto «Aggiorna» del Master, docs/PIANO_AGGIORNA.md, fase U3).
// Il server non aggiorna nulla da solo: avvia le operazioni pianificate di Windows create da installa.ps1
// (\GasterBless\Controlla e \GasterBless\Aggiorna), che lanciano strumenti\aggiorna_auto.ps1 con l'utente del Master.
// Questa pagina avvia, segue l'avanzamento leggendo l'esito scritto dallo script e, mentre Gaster Bless riparte,
// aspetta e si ricollega da sola. Solo dal PC di Gaster Bless (il server lo ricontrolla).
import { el, svuota, api, avviso, conferma, dialogo } from "../ui/ui.js";

// Ritmi dei controlli (le prove nel browser li accorciano).
export const tempi = { controllo: 2000, aggiorna: 3000, maxControllo: 120000, maxAggiorna: 20 * 60000 };

const PASSI = {
  avvio: "Avvio", controlli: "Controlli iniziali", novita: "Novità da GitHub", test: "Controlli automatici (qualche minuto)",
  compila: "Compilazione", installa: "Installazione: Gaster Bless si ferma per circa un minuto", verifica: "Controllo che Gaster Bless risponda",
};
const ESITI = {
  riuscito: ["✅", "Aggiornamento riuscito"], nessuna_novita: ["ℹ️", "Già aggiornata"], fallito: ["❌", "Non riuscito (la versione installata non è cambiata)"],
  ripristinato: ["⚠️", "Tornata alla versione precedente"], prova_ok: ["🧪", "Prova riuscita"], controllo: ["🔍", "Controllo delle novità"],
  in_corso: ["⏳", "In corso"],
};

const quando = (s) => {
  if (!s) return "";
  const d = new Date(s);
  return isNaN(d) ? s : d.toLocaleString("it-IT", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" });
};
const attendi = (ms) => new Promise((r) => setTimeout(r, ms));

export function sezioneAggiornamento(imp) {
  const box = el("section", { class: "scheda-ui agg", id: "aggiornamento" }, el("h2", { testo: "⟳ Aggiornamento di Gaster Bless" }));
  if (!imp.locale) {
    box.append(el("p", { class: "tenue", testo: "Gaster Bless si aggiorna da GitHub con un clic, ma solo dal PC su cui gira (per sicurezza): apri le Impostazioni da quel PC." }));
    return box;
  }
  const corpo = el("div", {}, el("p", { class: "tenue", testo: "Caricamento…" }));
  box.append(corpo);
  let st = null, lavoro = false;

  const elenco = (novita) => novita && novita.length
    ? el("ul", { class: "agg-novita" }, novita.map((n) => { const [h, ...r] = String(n).split(/\s+/); return el("li", {}, el("code", { testo: h }), " " + r.join(" ")); }))
    : null;

  function riquadroEsito(es, titolo) {
    if (!es) return null;
    const [ico, nome] = ESITI[es.esito] || ["•", es.esito || ""];
    return el("div", { class: "agg-esito agg-" + (es.esito || "") },
      el("div", {}, el("b", { testo: titolo + ": " }), ico + " " + nome, es.fine ? el("span", { class: "tenue", testo: " · " + quando(es.fine) }) : null),
      es.messaggio ? el("div", { class: "piccolo", testo: es.messaggio }) : null,
      es.esito === "controllo" || es.esito === "riuscito" ? elenco(es.novita) : null,
      es.registro ? el("button", { class: "piccolo agg-registro", testo: "📄 Vedi registro", on: { click: () => vediRegistro(es.soloControllo ? "controllo" : "") } }) : null);
  }

  async function vediRegistro(di) {
    try {
      const r = await api("/aggiornamento/registro" + (di ? "?di=" + di : ""));
      dialogo("Registro: " + r.nome, el("pre", { class: "agg-pre", testo: r.testo }), { largo: true });
    } catch (e) { avviso(e.message, "errore"); }
  }

  function disegna(stato, messaggio) {
    const pronto = stato && stato.pronto;
    svuota(corpo,
      el("p", { class: "tenue", testo: "Scarica da GitHub la versione nuova, la prova con i test automatici e la installa. Se la versione nuova non parte, Gaster Bless torna da sola alla precedente (programma e dati). Durante l'installazione Gaster Bless si ferma per circa un minuto: meglio non farlo a sessione in corso." }),
      stato ? el("p", { class: "piccolo tenue", testo: "Gaster Bless in funzione dal " + quando(stato.avviatoIl * 1000) + "." }) : null,
      !pronto && stato ? el("div", { class: "agg-esito agg-fallito" }, el("b", { testo: "Manca la preparazione una tantum. " }),
        "Clic destro su Start → «Terminale (amministratore)» e incolla, uno alla volta: ",
        el("pre", { class: "agg-pre", testo: "cd C:\\Progetti\\Progetto_GasterBless\ngit pull\npowershell -ExecutionPolicy Bypass -File C:\\Progetti\\Progetto_GasterBless\\04_nuova_app\\strumenti\\aggiorna.ps1" })) : null,
      messaggio || null,
      el("div", { class: "riga-flex" },
        el("button", { class: "agg-controlla", testo: "🔍 Controlla novità", disabled: lavoro || !(stato && stato.controlloPronto) || (stato && stato.inCorso), on: { click: controlla } }),
        el("button", { class: "primario agg-aggiorna", testo: "⟳ Aggiorna", disabled: lavoro || !pronto || (stato && stato.inCorso), on: { click: aggiorna } })),
      stato && stato.inCorso && !lavoro ? el("p", { class: "avviso-in-linea", testo: "⏳ C'è un aggiornamento in corso (avviato da qui o dal PC). Ricarica la pagina tra qualche minuto." }) : null,
      stato ? riquadroEsito(stato.ultimoControllo, "Ultimo controllo") : null,
      stato ? riquadroEsito(stato.ultimoAggiornamento, "Ultimo aggiornamento") : null);
  }

  async function carica() {
    try { st = await api("/aggiornamento"); disegna(st); }
    catch (e) { svuota(corpo, el("p", { class: "errore", testo: e.message })); }
  }

  // Avvia il controllo e aspetta che lo script scriva un controllo.json nuovo e finito.
  async function controlla() {
    const prima = st && st.ultimoControllo && st.ultimoControllo.inizio;
    lavoro = true;
    disegna(st, el("p", { class: "agg-attesa", testo: "🔍 Chiedo a GitHub le novità…" }));
    try {
      await api("/aggiornamento/controlla", { metodo: "POST", dati: {} });
      const fine = Date.now() + tempi.maxControllo;
      while (Date.now() < fine) {
        await attendi(tempi.controllo);
        try { st = await api("/aggiornamento"); } catch { continue; }
        const c = st.ultimoControllo;
        if (c && c.inizio !== prima && c.fine) break;
      }
      lavoro = false;
      const c = st.ultimoControllo;
      if (!c || c.inizio === prima) disegna(st, el("p", { class: "errore", testo: "Il controllo non ha risposto entro due minuti: sei collegato a Windows sul PC di Gaster Bless?" }));
      else disegna(st);
    } catch (e) { lavoro = false; disegna(st); avviso(e.message, "errore", 9000); }
  }

  // Avvia l'aggiornamento e ne segue i passi; mentre Gaster Bless riparte le richieste falliscono: si continua ad aspettare.
  async function aggiorna() {
    const c = st && st.ultimoControllo, n = c && c.esito === "controllo" ? (c.novita || []).length : -1;
    const testo = (n === 0 ? "L'ultimo controllo non ha trovato novità: verrà comunque fatto il controllo completo. " : n > 0 ? "Novità da installare: " + n + ". " : "")
      + "I test durano qualche minuto; poi Gaster Bless si ferma per circa un minuto e chi è collegato verrà scollegato (le pagine si ricollegano da sole). Aggiornare adesso?";
    if (!(await conferma("Aggiornare Gaster Bless?", testo, "⟳ Aggiorna adesso"))) return;
    const prima = st && st.ultimoAggiornamento && st.ultimoAggiornamento.inizio;
    lavoro = true;
    const riga = el("p", { class: "agg-attesa", testo: "⏳ Avvio dell'aggiornamento…" });
    disegna(st, riga);
    try { await api("/aggiornamento/avvia", { metodo: "POST", dati: {} }); }
    catch (e) { lavoro = false; disegna(st); avviso(e.message, "errore", 9000); return; }
    const fine = Date.now() + tempi.maxAggiorna;
    let visto = false, giu = false;
    while (Date.now() < fine) {
      await attendi(tempi.aggiorna);
      let r;
      try { r = await api("/aggiornamento"); } catch {
        giu = true;
        riga.textContent = "🔄 Gaster Bless si sta riavviando con la versione nuova… (la pagina aspetta e si ricollega da sola)";
        continue;
      }
      st = r;
      const a = r.ultimoAggiornamento;
      if (a && a.inizio !== prima) {
        visto = true;
        if (a.esito && a.esito !== "in_corso" && a.fine) break;
        riga.textContent = "⏳ " + (PASSI[a.passo] || a.passo || "In corso") + "…";
      } else if (!visto) riga.textContent = "⏳ In attesa che Windows avvii l'aggiornamento…";
      if (giu && !r.inCorso && visto) break; // riavviata: l'esito è già scritto
    }
    lavoro = false;
    const a = st && st.ultimoAggiornamento;
    if (!a || a.inizio === prima) {
      disegna(st, el("p", { class: "errore", testo: "L'aggiornamento non è partito: controlla di essere collegato a Windows sul PC di Gaster Bless e riprova." }));
      return;
    }
    disegna(st);
    const [ico, nome] = ESITI[a.esito] || ["", a.esito];
    avviso(ico + " " + nome, a.esito === "riuscito" || a.esito === "nessuna_novita" ? "" : "errore", 9000);
  }

  carica();
  return box;
}

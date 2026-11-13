# Scheda 5e dentro la nuova app

> Nota (13/11/2026): in GASTER BLESS – MASTER SUITE i dati di gioco sono stati tolti (nessun contenuto incluso) e i
> riferimenti alle vecchie cartelle di origine non valgono più: questa descrizione resta come guida alla struttura.

Copia della scheda di `01_scheda/sorgenti/` (che resta il riferimento, in sola lettura), servita come
pagina a sé in `/scheda/` e aperta in un riquadro dalla sezione Schede (`web/sezioni/schede.js`).
Comunica con l'app tramite il protocollo postMessage descritto in `01_scheda/INTEGRAZIONE.md`.

`index.html` = intestazione + `body.html` originale + `/regole/dati.js` + `/regole/motore.js` + `/regole/oggetti.js` + `scheda.js` + `oggetti_ui.js` + `avvio.js`.

**Oggetti magici (2026-11-10)**: `oggetti_ui.js` disegna il riquadro «✨ Oggetti magici» (equipaggia, sintonizza, attiva, cariche), l'editor guidato del Master (con anteprima e duplicazione), la scheda grafica dell'oggetto e le finestre di importazione/esportazione. I calcoli stanno in `/regole/oggetti.js` e in `scheda.js` (`abil`, `acCalc`, `speedCalc`, `initCalc`, `hpCalc`, `caster`, `renderAttacks`, tiri salvezza e abilità in `render`); gli oggetti sono in `st.mag`.
I dati delle regole (`data.js`, `spells1-4.js`, `featdesc.js`) sono in `/regole/dati.js`, condivisi con il
motore. `scheda.js` = `app.js`, `app2.js`, `app3.js`, `app4.js` uniti in quest'ordine, più la parte
"GASTER BLESS: MULTICLASSE" in fondo; `avvio.js` chiama `boot()`. Sono file e non script in linea, così vale la
Content-Security-Policy.

## Modifiche rispetto all'originale (tutte piccole e mirate)
| File | Modifica | Motivo |
|---|---|---|
| `scheda.js` (parte di `app.js`) | Dentro l'app non legge e non scrive `localStorage` | La scheda la salva il server; un PC condiviso non deve tenere copie |
| `scheda.js` (parte di `app3.js`) | pdf.js e mammoth da `/vendor/`; `isEvalSupported:false` | Funziona senza Internet; niente codice generato al volo |
| `scheda.js` (parte di `app3.js`) | Documenti Word ripuliti con DOMPurify | Prima c'erano solo espressioni regolari (punto S12 della Fase 1) |
| `scheda.js` (parte di `app3.js`) | Background fino a 15 MB | Uguale al limite del server |
| `scheda.js` (parte di `app4.js`) | html2canvas e jsPDF da `/vendor/` | Funziona senza Internet |
| `scheda.js` (parte di `app.js`) | `esc()` sul bonus magico dell'arma (`wp.m`) | Punto S13 della Fase 1: testo inserito nella pagina senza escape |
| `scheda.js` (parte di `app4.js`) | `loadState` controlla i dati (`validaStato`) | Punto S13: si tengono solo le chiavi note con il tipo giusto; il ritratto deve essere un'immagine |
| `style.css` | Caratteri Cinzel e Source Sans 3 da `/font/` | Niente Google Fonts |

Questa pagina ha una regola di sicurezza (CSP) leggermente diversa dal resto dell'app: ammette gli
stili scritti dentro l'HTML, che la scheda usa ovunque. Gli script restano ammessi solo da file.

## Multiclasse e regole (seconda tappa)
I calcoli che dipendono da classi e livelli passano da `/regole/motore.js` (vedi `web/regole/LEGGIMI.md`):
- livello del personaggio per bonus di competenza, soffio del dragonide, PF di nanica e Robusto; livello
  di ogni classe per i suoi privilegi (difesa senza armatura, movimento del monaco, Factotum, aura del
  paladino, attacco furtivo, ira, arti marziali, risorse...);
- riquadro **Multiclasse** nelle "Scelte del personaggio": classe, livello, sottoclasse, PF, requisiti
  e competenze ottenute; privilegi e aumenti di caratteristica di ogni classe;
- incantesimi: slot condivisi del multiclasse, patto separato, CD/attacco/conteggi per classe; nel
  selettore si sceglie per quale classe si prende l'incantesimo, oppure "Bonus" (non conta nei limiti);
- "+2 ?" sostituito da "⚠ Aumento dei punteggi di caratteristica: da scegliere".
Verifica: con una sola classe i numeri sono identici all'originale (2.280 casi, test in `test/regole/`).

## Prossimi passi
- Aggiungere da soli gli incantesimi sempre preparati delle sottoclassi (servono gli elenchi per ogni
  dominio, giuramento, patrono, circolo...) e i trucchetti razziali; per ora si aggiungono a mano come "Bonus".
- Estendere gli effetti automatici dei talenti (oggi: Allerta, Mobile, Osservatore, Resiliente, Robusto).

**Carta dell'oggetto (2026-11-11)**: `mgoCard` disegna la scheda dell'oggetto come una «carta» (classi CSS `.mgx-*` in fondo a
`style.css`) usando solo i campi esistenti; le regole di scrittura dei testi (una riga = una voce, «Intestazione: resto»,
«Nome - danni», `#` per un nuovo nastro) sono spiegate nel commento sopra `MGO_ICO` e nella nota della sezione C dell'editor.
I dati tecnici completi sono in `mgoDettagliTecnici`, nel riquadro «Dettagli dell'oggetto».

**Attacchi e inventario (2026-11-11, Fase A del combattimento)**: `armi_ui.js` (dopo `oggetti_ui.js`) disegna il riquadro
«🎒 Oggetti» (`st.inv`) e la finestra «⚙» di configurazione di una riga d'attacco. Il calcolo è in `scheda.js`
(`wpEff`, `calcolatoreAttacchi`, `calcolaWp`, `renderAttacks`, riepilogo `attacks`/`attacksPerAction`/`defenses`).

**Incantesimi (2026-11-11, fase F1)**: `incantesimi_ui.js` (dopo `armi_ui.js`) aggiunge alla finestra dell'incantesimo
il blocco «Effetto» (da `/regole/incantesimi_effetti.js`), la finestra della scelta dello slot e le regole del rituale
per classe; il lancio resta `lanciaIncantesimo` in `scheda.js`.

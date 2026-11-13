# Marchio: GASTER BLESS – MASTER SUITE

Deciso dal Master il 13/11/2026, con l'immagine di riferimento (intestazione scura con scritte oro). Sostituisce il nome
di lavoro «GM Manager». Il resto dell'app (funzioni, struttura, contenuti, colori delle pagine) **non cambia**: il marchio
riguarda solo nome, logo testuale e intestazione.

## Nome, slogan, sottotitolo
| Elemento | Testo | Uso |
|---|---|---|
| Nome ufficiale | **GASTER BLESS – MASTER SUITE** | titolo dell'app, finestre, installatore, guide, pagina delle licenze |
| Nome breve | **Gaster Bless** | icona sul telefono, spazi stretti |
| Slogan | **MASTER YOUR WORLD** | intestazione completa, copertine |
| Sottotitolo | **THE UNIVERSAL TABLETOP RPG PLATFORM** | intestazione completa, copertine, descrizione dell'app |

Sempre in maiuscolo nell'intestazione e nel logo (lo fa il CSS: nel codice si scrive in modo normale). Il trattino fra
«GASTER BLESS» e «MASTER SUITE» è lungo (–).

## Colori (presi dall'immagine di riferimento)
| Nome | Valore | Dove |
|---|---|---|
| `--gb-fondo` | `#191923` | fondo dell'intestazione e della barra |
| `--gb-oro` | `#d8b978` | GASTER BLESS, MASTER YOUR WORLD |
| `--gb-bianco` | `#ffffff` | MASTER SUITE |
| `--gb-filetto` | `#65543d` | linea sottile fra nome e slogan |
| `--gb-grigio` | `#bababb` | sottotitolo |

## Carattere
**Source Sans 3** (licenza SIL OFL 1.1, file in `branding/font/`): grassetto per nome e MASTER SUITE, seminero per lo
slogan, normale per il sottotitolo. Su Windows, se il file non si carica, il ripiego è Segoe UI, molto simile al
riferimento.

## File
| File | Contenuto |
|---|---|
| `marchio.css` | colori, carattere, intestazione completa (`.gb-intestazione`) e logo breve (`.gb-logo`) |
| `intestazione.html` | anteprima di entrambi: si apre con doppio clic nel browser |
| `logo.svg` | intestazione completa come immagine (copertine, documenti) |
| `font/` | Source Sans 3 e la sua licenza |

Intestazione completa:
```html
<div class="gb-intestazione">
  <h1 class="gb-nome">Gaster Bless</h1>
  <div class="gb-suite">Master Suite</div>
  <hr class="gb-filetto">
  <div class="gb-slogan">Master your world</div>
  <div class="gb-sottotitolo">The universal tabletop RPG platform</div>
</div>
```
Logo breve (barra in alto): `<a class="gb-logo" href="/"><span class="gb-nome">Gaster Bless</span><span class="gb-suite">Master Suite</span></a>`

## Dove va applicato (fase 3b, quando entra il codice)
Coerente in tutte le schermate, senza cambiare altro:
- titolo (`<title>`) di ogni pagina e della finestra di proiezione;
- `manifest.webmanifest`: `name` = nome ufficiale, `short_name` = «Gaster Bless»;
- barra in alto delle schermate interne: logo breve al posto del nome di Barovia;
- schermate d'accesso, registrazione, attesa dell'approvazione, benvenuto e caricamento: intestazione completa;
- messaggi e avvisi che citano il nome dell'app (inviti, notifiche, aggiornamento, errori);
- guide in PDF (copertina con l'intestazione, piè di pagina con il nome), PDF esportati dalla scheda;
- installatore, nome dell'eseguibile e del servizio, pagina «Informazioni» e pagina delle licenze.

Nomi tecnici interni (proposta, si possono cambiare): modulo Go `gasterbless`, eseguibile `GasterBless`.

## Da verificare prima della vendita
Il nome va controllato con l'avvocato insieme al marchio: «Gaster» è anche il nome di un personaggio noto di un
videogioco (W. D. Gaster di *Undertale*).

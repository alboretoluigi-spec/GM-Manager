# Contenuti e licenze di GASTER BLESS – MASTER SUITE

Registro di **tutto** ciò che entra nel prodotto e non è codice scritto per il progetto. Ogni voce nuova si aggiunge
qui **prima** del commit che la porta nel repository. Orientamento tecnico, non consulenza legale.

## 1. Cosa può entrare
| Tipo | Regola |
|---|---|
| Codice scritto per il progetto (anche quello nato in Barovia) | Sì, dopo la pulizia dei contenuti |
| Regole e dati di gioco (mostri, incantesimi, classi, oggetti, manuali, ambientazioni) | **Mai inclusi** (decisione del 13/11/2026): li importa o li inserisce l'utente. Le meccaniche sono codice nostro |
| Librerie, font, immagini, suoni di terzi | Solo con licenza che permetta la vendita, annotata al §3 |
| Materiale del Master (immagini, video, testi) | Solo con una dichiarazione scritta che ne ha i diritti |
| Contenuti dei manuali non SRD, ambientazioni, avventure, nomi di prodotti di terzi | **Mai** |

## 2. Attribuzione dell'SRD 5.1 (oggi non serve)
L'app non include dati dell'SRD. Il testo qui sotto serve solo se un giorno si decidesse di includerne (decisione del
Master); in quel caso va mostrato nell'app e nelle guide, nella forma richiesta dalla licenza:

> This work includes material taken from the System Reference Document 5.1 ("SRD 5.1") by Wizards of the Coast LLC
> and available at https://dnd.wizards.com/resources/systems-reference-document. The SRD 5.1 is licensed under the
> Creative Commons Attribution 4.0 International License available at
> https://creativecommons.org/licenses/by/4.0/legalcode.

Nel prodotto si può scrivere «compatibile con la quinta edizione»; non si usano i marchi né i loghi di Wizards of the
Coast.

## 3. Componenti di terzi (da verificare a ogni aggiunta)
| Componente | Versione | Licenza | Uso |
|---|---|---|---|
| modernc.org/sqlite | 1.61.1 | BSD-3 | database |
| github.com/coder/websocket | 1.8.15 | ISC | collegamento in tempo reale |
| golang.org/x/crypto, golang.org/x/sys | — | BSD-3 | cifratura, sistema |
| github.com/anthropics/anthropic-sdk-go | 2.7.0 | MIT | assistente IA (Claude) |
| rsc.io/qr | 0.2.0 | BSD-3 | codici QR |
| pdf.js | 3.11.174 | Apache 2.0 | lettura dei PDF |
| mammoth | 1.6.0 | BSD-2 | documenti Word |
| DOMPurify | 3.2.6 | Apache 2.0 / MPL 2.0 | pulizia dell'HTML |
| html2canvas | 1.4.1 | MIT | PDF della scheda |
| jsPDF | 2.5.1 | MIT | PDF della scheda |
| three.js | 0.170.0 | MIT | dadi 3D |
| Font Cinzel, Pirata One, Source Sans 3 | — | SIL OFL 1.1 | caratteri (Source Sans 3 è anche il carattere del marchio: `branding/font/`, con la licenza) |
| ffmpeg | — | (non incluso) | l'app lo cerca sul PC; se un giorno si includesse, rivedere LGPL/GPL |

Materiale creato per il progetto (13/11/2026), nessuna licenza di terzi: icone dell'app «GB» (`app/web/img/icona-*.png`),
video di benvenuto e immagine di saluto (dal marchio), immagine «borsa vuota», simbolo di stanchezza (`stanco.svg`),
suoni `corvo.mp3` e `attesa.mp3` (sintesi con ffmpeg), immagine provvisoria della mappa, guide PDF provvisorie.

Le dipendenze indirette di Go si elencano con `go list -m all` e si controllano prima della versione di prova (fase 6).

## 4. Elementi di Barovia esclusi
L'elenco completo è in `docs/PIANO.md` §3. Il 13/11/2026 il Master ha deciso di **togliere per ora** anche: icone delle
classi, video di benvenuto, immagini di saluto, del «povero» e del Monte Ghakis. Si sostituiscono con materiale nuovo,
con licenza annotata al §3.

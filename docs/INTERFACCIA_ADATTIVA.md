# Interfaccia adattiva al gioco scelto (fase 3c, passo C4): analisi e proposta

Richiesta del Master (13/11/2026): «Gaster Bless cambia l'interfaccia adattandola di volta in volta al gdr scelto dal
Master. Nasconde pulsanti e materiale non presenti nel gioco scelto e abilita pulsanti e funzioni riguardanti
esclusivamente il gioco scelto. Il Master seleziona il GDR quando fa accesso all'app.»

## 1. Inventario: sezioni e pulsanti di oggi
Legenda: **Tutti** = serve a qualsiasi gioco; **5e** = esiste solo per le meccaniche della quinta edizione (d20);
**Modulo** = si accende se il gioco lo prevede; **Adatta** = la sezione resta, ma il contenuto cambia con il gioco.

| Sezione o pulsante | Oggi | Con l'interfaccia adattiva |
|---|---|---|
| Sessioni, Documenti, Riassunti, Background | per tutti | **Tutti** |
| Mappa, PNG in scena (barra in alto) | per tutti | **Tutti** |
| Iniziativa | d20 + Destrezza, pareggi, schede dei mostri 5e | **Adatta**: il modo di tirare l'iniziativa lo decide il gioco (d20, carte, ordine libero…) |
| Dadi | preimpostati d20, vantaggio/svantaggio | **Adatta**: dadi e tiri del gioco (d20, d100, riserve di d6/d10, 2d6, dadi Fate…) |
| Schede | scheda della quinta edizione | **Adatta**: scheda del gioco; per i giochi senza modulo la **scheda libera** |
| Scheda di combattimento (dal tracker) | quinta edizione | **5e** (in futuro una per modulo) |
| Sanità mentale | regole facoltative della quinta edizione | **Modulo** «sanità/terrore»: per i giochi d'orrore o su richiesta |
| Finanze | monete mo/ma/mr | **Adatta**: valute del gioco (o una sola valuta libera) |
| Riposo lungo / Riposo breve (barra in alto) | quinta edizione | **5e** |
| Contenuti di gioco | pacchetti per «d20-5e» | **Adatta**: pacchetti del gioco scelto |
| Oggetti magici | formato della scheda 5e | **5e** |
| Strumenti e kit, Professioni | regole 5e | **5e** |
| PNG e mostri (bestiario) | schede con le statistiche 5e | **Adatta**: scheda della creatura del gioco (libera per i giochi senza modulo) |
| Manuali, Campagne, RPG Rulebook Library, My Rulebooks, Regole da approvare | per tutti | **Tutti** |
| Iscrizioni, Avvisi, Preparazione sessione, Assistente IA, Codice della campagna | per tutti | **Tutti** |
| Impostazioni, Profilo, Privacy, Scarica Guida | per tutti | **Tutti** |
| Dentro la scheda: indebolimento, ispirazione, slot, incantesimi, talenti | quinta edizione | **5e** (fanno parte della scheda di quel modulo) |

## 2. Giochi e moduli nella prima versione (proposta)
- **Quinta edizione (d20)**: tutto ciò che c'è oggi.
- **Scheda libera (qualsiasi altro gioco)**: le sezioni «Tutti»; scheda con campi e tiri definiti dal Master; dadi
  liberi; iniziativa in ordine libero o con un tiro scelto dal Master; valuta libera; bestiario con schede libere.
- Le altre famiglie del `docs/PIANO.md` §3.3bis (d100, riserve di dadi, 2d6, Fate…) diventano moduli successivi:
  ognuno dice quali sezioni accende, quale scheda, quali dadi e quale iniziativa.

## 3. Schermata di scelta all'accesso (proposta)
1. Il Master entra con nome e password (come oggi).
2. Compare **«Che cosa giochiamo oggi?»**: le campagne con il loro gioco (es. «Le terre del nord · Quinta edizione»)
   e, sotto, «Un altro gioco…» con i giochi disponibili. Si ricorda l'ultima scelta: basta un clic.
3. L'app si ridisegna: menu, barra in alto e pagine mostrano solo ciò che serve a quel gioco. Il nome del gioco resta
   visibile accanto al logo, con «Cambia gioco».
4. La scelta vale anche per i giocatori collegati: le loro pagine si aggiornano da sole.

## 4. Come si costruisce (sintesi tecnica)
- Un **registro dei moduli di sistema** (`web/sistemi/`): per ogni gioco id, nome, sezioni e pulsanti accesi, scheda,
  dadi, iniziativa, valute. Le sezioni del menu dichiarano a quali moduli appartengono.
- Il **gioco attivo** si salva sul server (impostazione del Master) e arriva a tutti con il canale in tempo reale.
- Le API delle sezioni spente rispondono «non disponibile per questo gioco», così nessuno le usa per sbaglio.
- Prove automatiche: per ogni modulo, il menu e la barra contengono solo le voci attese.

## 5. Decisioni del Master (13/11/2026)
1. All'accesso il Master sceglie **la campagna**; il gioco è quello della campagna.
2. I giocatori vedono **l'interfaccia scelta dal Master** (un solo gioco attivo alla volta; le loro pagine si aggiornano).
3. Nella prima versione: quinta edizione, scheda libera **e anche le altre famiglie** di meccaniche (§3.3bis del piano).
4. Le sezioni che non servono sono **nascoste del tutto** (menu, barra, pagine).

## 6. Sotto-passi
| Passo | Contenuto | Stato |
|---|---|---|
| C4a | Registro dei moduli di gioco (`app/web/sistemi/moduli.json`: tutte le famiglie, sezioni e pulsanti accesi); gioco di ogni campagna (migrazione 043); schermata «Che cosa giochiamo oggi?» all'accesso del Master; campagna attiva sul server; menu, barra e pagine filtrati per tutti; nome del gioco accanto al logo e «Cambia campagna»; giocatori aggiornati in tempo reale | in corso |
| C4b | Scheda libera: campi e tiri definiti dal Master (modelli di scheda), dadi liberi, iniziativa libera, valute del gioco nelle Finanze | da fare |
| C4c | Motore dei tiri per famiglia (d100 sotto il valore, riserve di d6 e d10 con successi, 2d6 + modificatore a fasce, d10 + caratteristica + abilità, dadi a passi esplosivi, dadi Fate, 3d6 sotto, d20 sotto, dadi narrativi) nella sezione Dadi e nella scheda | da fare |
| C4d | Modelli di scheda e iniziativa per ogni famiglia, costruiti sulla scheda libera (solo campi e calcoli, nessun testo dei manuali) | da fare |
| C4e | Bestiario con schede di creatura per famiglia, prove, guida | da fare |

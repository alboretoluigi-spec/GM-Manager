# Scheda libera (fase 3c, passo C4b)

Per i giochi che non hanno un modulo dedicato, il personaggio usa la **scheda libera**: il Master decide come è fatta,
campagna per campagna. Si usa per tutte le campagne con un gioco diverso dalla quinta edizione; per le famiglie di
meccaniche ancora in preparazione (`docs/INTERFACCIA_ADATTIVA.md` §6) vale finché il loro modulo non è pronto.

## Il modello della scheda
**Campagne → «📜 Modello della scheda…»** sulla campagna. Parte dal **modello di partenza del gioco della campagna**
(passo C4d, `app/internal/server/modelli_famiglie.go`): per ogni famiglia di meccaniche caratteristiche, abilità e
condizione con nomi comuni, tiri con la meccanica del gioco, punti ferita, difesa, iniziativa e valuta; per la scheda
libera generica Personaggio, Valori, Condizione, Equipaggiamento e note. Sono solo punti di partenza, senza elenchi
ricalcati da un gioco preciso: si adattano al gioco.
- **Sezioni**, ognuna con un titolo e i suoi **campi**. Per ogni campo: nome, identificativo (si calcola dal nome; serve
  nelle formule come `{identificativo}`), tipo, minimo e massimo, valore iniziale, spiegazione breve per i giocatori e,
  se serve, un **tiro con un clic** accanto al campo.
- **Tipi di campo**: testo breve, testo lungo, numero, risorsa (valore attuale e massimo), sì/no, elenco (una voce per riga).
- **Altri tiri**: pulsanti in cima alla scheda, con un nome e una formula.
- **Punti ferita**: l'identificativo del campo (risorsa o numero) che fa da punti ferita. Il valore attuale è quello
  che usa il tracker dell'iniziativa: danni e cure del tracker si vedono nella scheda.
- **Difesa**: l'identificativo del campo numerico che il tracker mostra come difesa.
- **Iniziativa**: la formula che il tracker usa quando il giocatore o il Master tirano l'iniziativa, oppure un **valore
  senza dadi** (es. `{destrezza}`) per un ordine fisso dal valore più alto. Vuota: l'iniziativa la scrive il Master.
- **Valute**: gli identificativi dei campi numerici con il denaro (es. `denaro`, `crediti`; al massimo 5). La sezione
  **Finanze** li mostra: il Master dà o toglie (mai sotto zero), i giocatori trasferiscono ad altri personaggi che hanno
  la stessa valuta; ogni movimento va nel registro e nella scheda.

Il modello si **esporta** e si **importa** da file (JSON) e si può tornare al **modello di base**. Ogni salvataggio è
controllato: un errore (identificativo ripetuto, formula sbagliata, campo che non esiste) si vede nella finestra e il
modello resta com'era.

## Le formule
- Dadi: `d4`, `d6`, `d8`, `d10`, `d12`, `d20`, `d100`, da 1 a 20 per tipo (al massimo 50 in tutto): `2d6`, `d20`, `3d10`.
- Numeri e campi numerici tra graffe: `1d20 + {forza} - 2`, `2d6 + {abilita}`.
- Il numero dei dadi può venire da un campo: `{forza}d6` tira tanti d6 quanto vale Forza.
- Il risultato è la somma.
- **Meccanica del gioco** (C4c, `docs/MECCANICHE.md`): un campo con «tiro con la meccanica del gioco» oppure un tiro
  del modello con un **valore** al posto della formula (es. `{furtivita} + 10`) apre il modulo del tiro del gioco della
  campagna (d100, riserve, 2d6 a fasce…) con il valore già scritto; l'esito torna nella scheda.

## La scheda del giocatore
Si apre dalla sezione Schede come l'altra scheda: salvataggio automatico, versioni, approvazione del Master (con
l'approvazione attiva i dati si cambiano con «Modifica manuale»; punti ferita e tiri restano liberi), copia sul
dispositivo senza connessione. I tiri li fa il server e si vedono anche con i dadi 3D.

## Finanze
Nella quinta edizione le Finanze usano le monete della sua scheda (acquisti dall'elenco, professioni, ricompense); negli
altri giochi le valute del modello (sopra).

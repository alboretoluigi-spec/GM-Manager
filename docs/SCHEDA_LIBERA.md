# Scheda libera (fase 3c, passo C4b)

Per i giochi che non hanno un modulo dedicato, il personaggio usa la **scheda libera**: il Master decide come è fatta,
campagna per campagna. Si usa per tutte le campagne con un gioco diverso dalla quinta edizione; per le famiglie di
meccaniche ancora in preparazione (`docs/INTERFACCIA_ADATTIVA.md` §6) vale finché il loro modulo non è pronto.

## Il modello della scheda
**Campagne → «📜 Modello della scheda…»** sulla campagna. Parte dal modello di base dell'app (Personaggio, Valori,
Condizione, Equipaggiamento e note) e si adatta al gioco:
- **Sezioni**, ognuna con un titolo e i suoi **campi**. Per ogni campo: nome, identificativo (si calcola dal nome; serve
  nelle formule come `{identificativo}`), tipo, minimo e massimo, valore iniziale, spiegazione breve per i giocatori e,
  se serve, un **tiro con un clic** accanto al campo.
- **Tipi di campo**: testo breve, testo lungo, numero, risorsa (valore attuale e massimo), sì/no, elenco (una voce per riga).
- **Altri tiri**: pulsanti in cima alla scheda, con un nome e una formula.
- **Punti ferita**: l'identificativo del campo (risorsa o numero) che fa da punti ferita. Il valore attuale è quello
  che usa il tracker dell'iniziativa: danni e cure del tracker si vedono nella scheda.
- **Difesa**: l'identificativo del campo numerico che il tracker mostra come difesa.
- **Iniziativa**: la formula che il tracker usa quando il giocatore o il Master tirano l'iniziativa. Vuota: l'iniziativa
  la scrive il Master.

Il modello si **esporta** e si **importa** da file (JSON) e si può tornare al **modello di base**. Ogni salvataggio è
controllato: un errore (identificativo ripetuto, formula sbagliata, campo che non esiste) si vede nella finestra e il
modello resta com'era.

## Le formule
- Dadi: `d4`, `d6`, `d8`, `d10`, `d12`, `d20`, `d100`, da 1 a 20 per tipo (al massimo 50 in tutto): `2d6`, `d20`, `3d10`.
- Numeri e campi numerici tra graffe: `1d20 + {forza} - 2`, `2d6 + {abilita}`.
- Il numero dei dadi può venire da un campo: `{forza}d6` tira tanti d6 quanto vale Forza.
- Il risultato è la somma. Successi, soglie e dadi speciali delle singole famiglie arrivano con i loro moduli (C4c).

## La scheda del giocatore
Si apre dalla sezione Schede come l'altra scheda: salvataggio automatico, versioni, approvazione del Master (con
l'approvazione attiva i dati si cambiano con «Modifica manuale»; punti ferita e tiri restano liberi), copia sul
dispositivo senza connessione. I tiri li fa il server e si vedono anche con i dadi 3D.

## Cosa resta della quinta edizione
Le Finanze usano le monete della quinta edizione: per ora compaiono solo in quel gioco. Le valute del gioco arriveranno
in un passo successivo.

# Tiri delle famiglie di meccaniche (fase 3c, passo C4c)

Ogni gioco del registro (`app/web/sistemi/moduli.json`, campo `tiro`) ha il suo tiro. Lo fa il server
(`app/internal/server/meccaniche.go`, messaggio «dadi.meccanica»): i dadi si vedono come gli altri tiri (anche in 3D) e
**l'esito compare in parole** sotto il risultato, negli avvisi e nel registro dello scontro. Sono meccaniche generiche
scritte da noi, con nomi descrittivi; nessun testo dei manuali.

Dove si usa:
- **sezione Dadi**: in cima c'è il pannello del gioco della campagna attiva, sotto i dadi liberi;
- **scheda libera**: un campo con «tiro con la meccanica del gioco» o un tiro del modello con un «valore» aprono il
  pannello con il valore già scritto (`docs/SCHEDA_LIBERA.md`).

| Meccanica | Si inserisce | Come si legge |
|---|---|---|
| **d100 sotto il valore** (`d100`) | valore, modificatore, dadi bonus o penalità (fino a 2: si tirano più decine, si tiene la migliore o la peggiore) | 1 successo critico; fino a un quinto del valore successo estremo; fino a metà successo difficile; fino al valore successo; sopra fallimento; 100 (o 96-100 con valore sotto 50) disastro |
| **Riserva di d10** (`riserva-d10`) | dadi, modificatore (dadi in più o in meno), soglia del successo (6), successi richiesti (1); facoltativo: due 10 valgono due successi in più | successi contati; successo se bastano, critico con le coppie di 10 |
| **Riserva di d6** (`riserva-d6`) | come sopra (soglia 6); oppure «vale il dado più alto»; facoltativo: disastro senza successi e con un 1 | dado più alto: 6 successo pieno, due 6 critico, 4-5 parziale, 1-3 fallimento; senza dadi se ne tirano 2 e vale il più basso |
| **2d6 a fasce** (`2d6-fasce`) | modificatore | 10 o più successo pieno, 7-9 parziale, 6 o meno fallimento |
| **d10 + valore** (`d10-somma`) | valore (caratteristica + abilità), modificatore, difficoltà (15) | con 10 si tira un altro d10 e si somma (critico se riesce), con 1 se ne tira un altro e si sottrae (fallimento critico se non riesce); successo se si raggiunge la difficoltà |
| **Dado esplosivo** (`passi-esplosivi`) | dado del tratto (d4-d12), modificatore, numero da raggiungere (4); facoltativo: dado jolly d6 | il massimo si ritira e si somma; con il jolly vale il migliore; ogni 4 in più un incremento; 1 su entrambi i dadi disastro |
| **Dadi Fate** (`fate`) | abilità, modificatore, opposizione (0) | 4 dadi con −, vuoto, + (sullo schermo d6: 1-2 −, 3-4 vuoto, 5-6 +); scarto 3 o più successo con stile, 1-2 successo, 0 pareggio, sotto fallimento |
| **3d6 sotto il valore** (`3d6-sotto`) | valore, modificatore | 3-4 (5 con valore 15, 6 con 16) successo critico; 18 (17 con valore fino a 15, o 10 sotto il valore) fallimento critico; altrimenti successo o fallimento con il margine |
| **d20 sotto il valore** (`d20-sotto`) | valore, modificatore, vantaggio o svantaggio (due d20: il più basso o il più alto) | 1 successo critico, 20 fallimento critico, sotto o uguale al valore successo |
| **Dadi narrativi** (`narrativi`) | numero di dadi per tipo: abilità (d8), competenza (d12), aiuto (d6), difficoltà (d8), sfida (d12), ostacolo (d6) | successi meno fallimenti; vantaggi e minacce si annullano; trionfo e disperazione |

Quinta edizione e scheda libera non hanno un tiro di famiglia: usano i dadi liberi, i tiri della scheda e le formule.
La «Tabella della sfortuna» (1 naturale sul d20) resta solo in questi due giochi.

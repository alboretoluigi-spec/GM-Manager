# Istruzioni permanenti per Claude: GASTER BLESS – MASTER SUITE

Leggi questo file all'inizio di ogni sessione. Vale per qualsiasi account usato su questo repository.

## Il progetto
**GASTER BLESS – MASTER SUITE** (nome breve «Gaster Bless»; nome di lavoro precedente «GM Manager», repository
`GM-Manager`) è un'app per il Master di giochi di ruolo **generica, per diversi
sistemi di gioco**, da vendere con licenza e da installare sul PC del Master (Windows e macOS). I giocatori si collegano dal browser (telefono, tablet, PC). Nasce dal codice di
**Barovia**, l'app personale del Master per la sua campagna, ma è un prodotto **separato e generico**: nessuna campagna,
nessun contenuto di terzi, **nessun contenuto di gioco incluso**.

Piano, decisioni e inventario: `docs/PIANO.md`. Marchio (nome, slogan, colori, logo): `branding/MARCHIO.md`. Regole sui contenuti e le licenze: `docs/LICENZE_E_CONTENUTI.md`.

## Prima di tutto, a ogni sessione
1. Leggi `STATO_LAVORO.md`: è la memoria condivisa tra gli account. Riparti da «Prossimo passo».
2. Non dare per scontato nulla che non sia scritto lì o nei file del repository.

## Barovia è solo un riferimento
Il codice di partenza viene dal repository `alboretoluigi-spec/Progetto_Barovia` (cartella `04_nuova_app`). Da lì si
**copia**, mai il contrario: in Barovia non si scrive nulla per conto di questo progetto. Si copia solo ciò che le regole di
`docs/LICENZE_E_CONTENUTI.md` permettono, e la storia Git di Barovia non entra mai qui.

## Regole sui contenuti (vincolanti)
- Mai nel prodotto (codice, pagine, dati, immagini, guide): nomi, testi, mappe, immagini o personaggi di prodotti di
  Wizards of the Coast o di altri editori (per esempio Curse of Strahd, Barovia, Strahd, Ravenloft, Tarokka); i marchi
  «Dungeons & Dragons» e «D&D». I documenti interni in `docs/` possono nominarli solo per spiegare cosa escludere.
- **Nessun contenuto di gioco nell'app** (decisione del Master, 13/11/2026): niente mostri, incantesimi, classi, oggetti,
  manuali, ambientazioni o avventure inclusi o scaricati automaticamente dall'app, nemmeno dall'SRD. L'app offre gli
  strumenti perché l'utente li **importi da file che si procura lui** o li **inserisca a mano**.
- Le meccaniche (calcoli, tiri, iniziativa, condizioni) sono codice nostro e si organizzano per **sistema di gioco**,
  senza testi dei manuali.
- Immagini, suoni, font e librerie: solo con una licenza che permetta la vendita, annotata in `docs/LICENZE_E_CONTENUTI.md`.
- Niente dati personali del Master o dei suoi giocatori nel repository.
- In caso di dubbio su un contenuto: non copiarlo, segnalalo e chiedi.

## Regole di lavoro
- Rispondi sempre in italiano, in modo chiaro e senza gergo inutile.
- Lavora a fasi, come descritto in `docs/PIANO.md`. Alla fine di ogni fase fermati e chiedi: «Confermi che posso
  procedere con la fase successiva?».
- Non installare, non configurare servizi esterni (pagamenti, domini, firme del codice, account) senza conferma esplicita.
- Prima di modifiche importanti fai un commit di ripristino.
- Regole di gioco: solo sintesi scritte con parole tue o testo dell'SRD 5.1, mai il testo dei manuali.
- Se una decisione non è reversibile o ha più alternative valide, chiedi.
- Comandi per il PC del Master: sempre per intero, uno per blocco (PowerShell).

## Fine di ogni sessione (obbligatorio)
Prima di chiudere, oppure quando l'utente scrive «chiudi sessione», aggiorna `STATO_LAVORO.md`:
- aggiungi una riga al **Registro sessioni** (data, account se indicato, cosa hai fatto);
- aggiorna **Fase attuale**, **Decisioni prese**, **Problemi aperti** e **Prossimo passo**;
- elenca i file creati o modificati.

Scrivi in modo che un'altra sessione, senza nessun altro contesto, possa riprendere subito.

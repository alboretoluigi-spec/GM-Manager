# GASTER BLESS – MASTER SUITE: il programma

Server (Go) e pagine (HTML/JS) dell'app. Il programma gira sul PC del Master; i giocatori lo aprono dal browser.

| Cartella | Contenuto |
|---|---|
| `cmd/gasterbless` | avvio del programma |
| `internal/server` | API, tempo reale, assistente IA, mappa, iniziativa, schede, compendio |
| `internal/db` | database SQLite e migrazioni (`schema/`, `rollback/`) |
| `internal/config`, `internal/cripto`, `internal/rete` | configurazione, cifratura, rete di casa |
| `internal/controlli` | prove su tutto il prodotto (parole vietate: `../docs/parole_vietate.txt`) |
| `web` | pagine, stili (`stile/marchio.css` per il marchio), librerie di terzi in `vendor/` |
| `guide` | guide in PDF incorporate (oggi provvisorie) |
| `strumenti` | script per Windows ereditati (da rifare nella fase 5) |

Comandi (dalla cartella `app`):
```
go build ./cmd/gasterbless
go test ./...
```

Nessun contenuto di gioco è incluso: bestiario, regole, mappe e manuali li importa o li inserisce il Master
(`../docs/PIANO.md`). Stato delle prove: `../STATO_LAVORO.md`.

-- Schema iniziale di Gaster Bless. Tempi in secondi Unix (UTC). Identificativi casuali (testo).

-- ===== Account, accesso, consensi =====
CREATE TABLE utenti (
  id                 TEXT PRIMARY KEY,
  nome               TEXT NOT NULL,
  telefono_cifrato   BLOB,                 -- AES-GCM, contesto "telefono:<id>"
  telefono_impronta  TEXT UNIQUE,          -- HMAC del numero normalizzato (doppioni)
  telefono_finale    TEXT,                 -- ultime 4 cifre, per la vista mascherata
  password_hash      TEXT NOT NULL,        -- Argon2id (PHC)
  ruolo              TEXT NOT NULL CHECK (ruolo IN ('master','giocatore')),
  stato              TEXT NOT NULL CHECK (stato IN ('in_attesa','approvato','rifiutato','bloccato','sospeso','cancellazione_richiesta')),
  avvisi_whatsapp    INTEGER NOT NULL DEFAULT 1,
  totp_cifrato       BLOB,
  totp_ultimo        INTEGER NOT NULL DEFAULT 0, -- ultimo intervallo usato (anti-riuso)
  creato_il          INTEGER NOT NULL,
  modificato_il      INTEGER NOT NULL,
  ultimo_accesso     INTEGER
);
CREATE UNIQUE INDEX un_solo_master ON utenti(ruolo) WHERE ruolo = 'master';

CREATE TABLE consensi (
  id         INTEGER PRIMARY KEY,
  utente_id  TEXT NOT NULL REFERENCES utenti(id) ON DELETE CASCADE,
  versione   TEXT NOT NULL,
  accettato_il INTEGER NOT NULL
);

CREATE TABLE inviti (
  id          TEXT PRIMARY KEY,
  codice_hash TEXT NOT NULL UNIQUE,
  gruppo_id   TEXT REFERENCES gruppi(id) ON DELETE SET NULL,
  nota        TEXT NOT NULL DEFAULT '',
  scade_il    INTEGER,
  usi_max     INTEGER,
  usi         INTEGER NOT NULL DEFAULT 0,
  revocato    INTEGER NOT NULL DEFAULT 0,
  creato_il   INTEGER NOT NULL
);

CREATE TABLE sessioni_accesso (
  token_hash  TEXT PRIMARY KEY,
  utente_id   TEXT NOT NULL REFERENCES utenti(id) ON DELETE CASCADE,
  csrf        TEXT NOT NULL,
  creato_il   INTEGER NOT NULL,
  scade_il    INTEGER NOT NULL,
  ultimo_uso  INTEGER NOT NULL,
  ip          TEXT NOT NULL DEFAULT '',
  dispositivo TEXT NOT NULL DEFAULT '',
  totp_ok     INTEGER NOT NULL DEFAULT 0   -- secondo fattore superato (Master)
);
CREATE INDEX sessioni_per_utente ON sessioni_accesso(utente_id);

CREATE TABLE dispositivi_noti (
  utente_id  TEXT NOT NULL REFERENCES utenti(id) ON DELETE CASCADE,
  impronta   TEXT NOT NULL,
  descrizione TEXT NOT NULL DEFAULT '',
  primo_il   INTEGER NOT NULL,
  ultimo_il  INTEGER NOT NULL,
  PRIMARY KEY (utente_id, impronta)
);

CREATE TABLE tentativi_accesso (
  chiave        TEXT PRIMARY KEY,   -- "ip:<indirizzo>" oppure "tel:<impronta>"
  falliti       INTEGER NOT NULL DEFAULT 0,
  blocchi       INTEGER NOT NULL DEFAULT 0,
  bloccato_fino INTEGER NOT NULL DEFAULT 0,
  aggiornato_il INTEGER NOT NULL
);

CREATE TABLE codici_reimpostazione (
  id          TEXT PRIMARY KEY,
  utente_id   TEXT NOT NULL REFERENCES utenti(id) ON DELETE CASCADE,
  codice_hash TEXT NOT NULL UNIQUE,
  scade_il    INTEGER NOT NULL,
  usato_il    INTEGER,
  creato_il   INTEGER NOT NULL
);

-- ===== Gruppi e sessioni di gioco =====
CREATE TABLE gruppi (
  id         TEXT PRIMARY KEY,
  nome       TEXT NOT NULL,
  giorno     TEXT NOT NULL DEFAULT '',
  colore     TEXT NOT NULL DEFAULT '',
  archiviato INTEGER NOT NULL DEFAULT 0,
  creato_il  INTEGER NOT NULL
);

CREATE TABLE iscrizioni (
  utente_id  TEXT NOT NULL REFERENCES utenti(id) ON DELETE CASCADE,
  gruppo_id  TEXT NOT NULL REFERENCES gruppi(id) ON DELETE CASCADE,
  segretario INTEGER NOT NULL DEFAULT 0,
  dal        INTEGER NOT NULL,
  PRIMARY KEY (utente_id, gruppo_id)
);

CREATE TABLE sessioni_gioco (
  id          TEXT PRIMARY KEY,
  gruppo_id   TEXT NOT NULL REFERENCES gruppi(id) ON DELETE CASCADE,
  titolo      TEXT NOT NULL,
  inizio_il   INTEGER NOT NULL,          -- data e ora (UTC)
  luogo       TEXT NOT NULL DEFAULT '',
  note        TEXT NOT NULL DEFAULT '',
  stato       TEXT NOT NULL DEFAULT 'pianificata' CHECK (stato IN ('pianificata','in_corso','conclusa')),
  mqtt_remoto INTEGER NOT NULL DEFAULT 0,
  creato_il   INTEGER NOT NULL
);
CREATE INDEX sessioni_per_gruppo ON sessioni_gioco(gruppo_id, inizio_il);

CREATE TABLE partecipanti (
  sessione_id    TEXT NOT NULL REFERENCES sessioni_gioco(id) ON DELETE CASCADE,
  utente_id      TEXT NOT NULL REFERENCES utenti(id) ON DELETE CASCADE,
  personaggio_id TEXT REFERENCES personaggi(id) ON DELETE SET NULL,
  confermato     INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (sessione_id, utente_id)
);

-- ===== Personaggi e schede =====
CREATE TABLE personaggi (
  id                TEXT PRIMARY KEY,
  proprietario_id   TEXT REFERENCES utenti(id) ON DELETE SET NULL,
  gruppo_id         TEXT REFERENCES gruppi(id) ON DELETE SET NULL,
  nome              TEXT NOT NULL,
  stato             TEXT NOT NULL DEFAULT 'attivo' CHECK (stato IN ('attivo','ritirato','morto','archiviato','anonimo')),
  ritratto          TEXT NOT NULL DEFAULT '',  -- nome file nella cartella dati
  versione_corrente INTEGER NOT NULL DEFAULT 0,
  creato_il         INTEGER NOT NULL,
  modificato_il     INTEGER NOT NULL
);

CREATE TABLE schede_versioni (
  personaggio_id TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  numero         INTEGER NOT NULL,
  dati           TEXT NOT NULL,   -- JSON completo della scheda
  riepilogo      TEXT NOT NULL,   -- JSON: ca, pfMax, init, des, percPassiva, livello, classi
  campi          TEXT NOT NULL DEFAULT '[]', -- percorsi cambiati rispetto alla versione precedente
  autore_id      TEXT,
  nota           TEXT NOT NULL DEFAULT '',
  creato_il      INTEGER NOT NULL,
  PRIMARY KEY (personaggio_id, numero)
);

CREATE TABLE stato_in_gioco (
  personaggio_id TEXT PRIMARY KEY REFERENCES personaggi(id) ON DELETE CASCADE,
  pf             INTEGER,
  pf_temp        INTEGER NOT NULL DEFAULT 0,
  ts_morte       TEXT NOT NULL DEFAULT '{"s":0,"f":0}',
  condizioni     TEXT NOT NULL DEFAULT '[]',
  slot_usati     TEXT NOT NULL DEFAULT '{}',
  risorse_usate  TEXT NOT NULL DEFAULT '{}',
  ispirazione    INTEGER NOT NULL DEFAULT 0,
  modificato_il  INTEGER NOT NULL,
  modificato_da  TEXT
);

CREATE TABLE background (
  personaggio_id TEXT PRIMARY KEY REFERENCES personaggi(id) ON DELETE CASCADE,
  file           TEXT NOT NULL,   -- nome generato dal server
  tipo           TEXT NOT NULL CHECK (tipo IN ('pdf','docx')),
  nome_originale TEXT NOT NULL,
  dimensione     INTEGER NOT NULL,
  sha256         TEXT NOT NULL,
  caricato_da    TEXT,
  caricato_il    INTEGER NOT NULL
);

-- ===== Mappa, contenuti, bestiario, musica =====
CREATE TABLE stato_mappa (
  gruppo_id      TEXT PRIMARY KEY REFERENCES gruppi(id) ON DELETE CASCADE,
  nebbia         BLOB NOT NULL,        -- 1 bit per cella (81 x 53 celle da 20 px)
  scala          REAL NOT NULL DEFAULT 0.25,
  nomi_svelati   TEXT NOT NULL DEFAULT '{}',
  media_svelati  TEXT NOT NULL DEFAULT '{}',
  mappe_nascoste TEXT NOT NULL DEFAULT '{}',
  proiezione     TEXT NOT NULL DEFAULT '{"kind":"none"}',
  mostrati       TEXT NOT NULL DEFAULT '[]', -- immagini del bestiario aperte sui telefoni
  modificato_il  INTEGER NOT NULL
);

CREATE TABLE localita_contenuti (
  localita  TEXT PRIMARY KEY,
  descrizione TEXT NOT NULL DEFAULT '',
  note_master TEXT NOT NULL DEFAULT ''
);

CREATE TABLE media (
  id        TEXT PRIMARY KEY,
  localita  TEXT NOT NULL,
  tipo      TEXT NOT NULL CHECK (tipo IN ('img','vid','link')),
  file      TEXT NOT NULL DEFAULT '',  -- percorso relativo alla cartella media
  url       TEXT NOT NULL DEFAULT '',
  didascalia TEXT NOT NULL DEFAULT '',
  ordine    INTEGER NOT NULL,
  creato_il INTEGER NOT NULL
);
CREATE INDEX media_per_localita ON media(localita, ordine);

CREATE TABLE manuali (
  id        TEXT PRIMARY KEY,
  nome      TEXT NOT NULL,
  tipo      TEXT NOT NULL CHECK (tipo IN ('mm','avventura','altro')),
  file      TEXT NOT NULL,
  ref_libro INTEGER NOT NULL DEFAULT 1,
  ref_pdf   INTEGER NOT NULL DEFAULT 1,
  doppia    INTEGER NOT NULL DEFAULT 0,
  indice    TEXT NOT NULL DEFAULT '{}',
  indicizzato INTEGER NOT NULL DEFAULT 0,
  senza_testo INTEGER NOT NULL DEFAULT 0,
  creato_il INTEGER NOT NULL
);

CREATE TABLE bestiario_voci (
  id     TEXT PRIMARY KEY,
  nome   TEXT NOT NULL,
  cat    TEXT NOT NULL CHECK (cat IN ('png','mostro')),
  ruolo  TEXT NOT NULL DEFAULT '',
  creato_il INTEGER NOT NULL
);

CREATE TABLE bestiario_personalizzazioni (
  voce_id  TEXT PRIMARY KEY,
  immagine TEXT NOT NULL DEFAULT '',
  note     TEXT NOT NULL DEFAULT '',
  scheda   TEXT NOT NULL DEFAULT '',
  manuale_id TEXT,
  pagina   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE impostazioni (
  chiave TEXT PRIMARY KEY,
  valore TEXT NOT NULL
);

-- ===== Iniziativa =====
CREATE TABLE combattimenti (
  id          TEXT PRIMARY KEY,
  sessione_id TEXT NOT NULL REFERENCES sessioni_gioco(id) ON DELETE CASCADE,
  round       INTEGER NOT NULL DEFAULT 0,
  turni       INTEGER NOT NULL DEFAULT 0,
  turno_di    TEXT,
  attivo      INTEGER NOT NULL DEFAULT 1,
  creato_il   INTEGER NOT NULL
);
CREATE UNIQUE INDEX un_combattimento_attivo ON combattimenti(sessione_id) WHERE attivo = 1;

CREATE TABLE combattenti (
  id              TEXT PRIMARY KEY,
  combattimento_id TEXT NOT NULL REFERENCES combattimenti(id) ON DELETE CASCADE,
  tipo            TEXT NOT NULL CHECK (tipo IN ('pg','mostro')),
  personaggio_id  TEXT REFERENCES personaggi(id) ON DELETE CASCADE,
  nome            TEXT NOT NULL,
  iniziativa      INTEGER,
  spareggio       INTEGER NOT NULL DEFAULT 0,  -- Destrezza
  nascosto        INTEGER NOT NULL DEFAULT 0,
  ca              INTEGER NOT NULL DEFAULT 10, -- solo mostri
  pf              INTEGER NOT NULL DEFAULT 1,  -- solo mostri
  pf_max          INTEGER NOT NULL DEFAULT 1,  -- solo mostri
  condizioni      TEXT NOT NULL DEFAULT '[]',  -- solo mostri
  creato_il       INTEGER NOT NULL
);

CREATE TABLE registro_combattimento (
  id          INTEGER PRIMARY KEY,
  combattimento_id TEXT NOT NULL REFERENCES combattimenti(id) ON DELETE CASCADE,
  autore_id   TEXT,
  testo       TEXT NOT NULL,
  creato_il   INTEGER NOT NULL
);

CREATE TABLE messaggi (
  id          TEXT PRIMARY KEY,
  sessione_id TEXT NOT NULL REFERENCES sessioni_gioco(id) ON DELETE CASCADE,
  mittente_id TEXT REFERENCES utenti(id) ON DELETE SET NULL,
  destinatario_id TEXT REFERENCES utenti(id) ON DELETE SET NULL,
  testo       TEXT NOT NULL,
  letto       INTEGER NOT NULL DEFAULT 0,
  creato_il   INTEGER NOT NULL
);
CREATE INDEX messaggi_per_sessione ON messaggi(sessione_id, creato_il);

-- ===== Avvisi, privacy =====
CREATE TABLE modelli_avviso (
  id    TEXT PRIMARY KEY,
  nome  TEXT NOT NULL,
  testo TEXT NOT NULL
);

CREATE TABLE avvisi (
  id          TEXT PRIMARY KEY,
  sessione_id TEXT NOT NULL REFERENCES sessioni_gioco(id) ON DELETE CASCADE,
  testo_modello TEXT NOT NULL,
  creato_da   TEXT,
  creato_il   INTEGER NOT NULL
);

CREATE TABLE avvisi_destinatari (
  avviso_id  TEXT NOT NULL REFERENCES avvisi(id) ON DELETE CASCADE,
  utente_id  TEXT NOT NULL REFERENCES utenti(id) ON DELETE CASCADE,
  testo      TEXT NOT NULL,
  stato      TEXT NOT NULL DEFAULT 'da_inviare' CHECK (stato IN ('da_inviare','inviato','saltato')),
  segnato_il INTEGER,
  PRIMARY KEY (avviso_id, utente_id)
);

CREATE TABLE richieste_privacy (
  id         TEXT PRIMARY KEY,
  utente_id  TEXT REFERENCES utenti(id) ON DELETE SET NULL,
  nome       TEXT NOT NULL,
  tipo       TEXT NOT NULL CHECK (tipo IN ('cancellazione')),
  stato      TEXT NOT NULL DEFAULT 'aperta' CHECK (stato IN ('aperta','eseguita','annullata')),
  creato_il  INTEGER NOT NULL,
  chiuso_il  INTEGER
);

CREATE TABLE registro_dati_personali (
  id        INTEGER PRIMARY KEY,
  attore_id TEXT,
  azione    TEXT NOT NULL,     -- per esempio "telefono_visto", "esportazione", "cancellazione"
  utente_interessato TEXT,
  creato_il INTEGER NOT NULL
);

CREATE TABLE registro_eventi (
  id        INTEGER PRIMARY KEY,
  tipo      TEXT NOT NULL,     -- "accesso", "accesso_fallito", "nuovo_dispositivo", "sistema"...
  utente_id TEXT,
  dettaglio TEXT NOT NULL DEFAULT '',
  creato_il INTEGER NOT NULL
);

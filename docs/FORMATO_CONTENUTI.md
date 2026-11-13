# Formato dei pacchetti di contenuti «gasterbless-contenuti» (versione 1)

GASTER BLESS – MASTER SUITE non contiene contenuti di gioco. Classi, razze, background, incantesimi, armi, armature,
talenti e stili di combattimento arrivano da **pacchetti** che l'utente prepara o si procura e importa dalla pagina
**Campagna → Contenuti di gioco**. Questo documento descrive il formato: è aperto, chiunque può scrivere un pacchetto
con un editor di testo.

- Un pacchetto è un file di testo **JSON** in UTF-8 (estensione `.json`), al massimo 20 MB.
- Modello completo con contenuti inventati: pulsante «⬇ Scarica il modello» nella pagina (file
  `app/web/modelli/modello_pacchetto.json`).
- Prima di importare, l'app controlla tutto il file e indica ogni errore con il punto preciso
  (per esempio «classi n. 2 («Spadaccino»): «dadoVita» deve essere tra 4 e 20»). Un file con errori non viene importato.
- I nomi dei campi sono quelli delle tabelle qui sotto; un campo scritto male viene segnalato («campo sconosciuto»).
  I campi non indicati come obbligatori si possono omettere.
- **Importa solo materiale che hai il diritto di usare.** I testi delle descrizioni devono essere tuoi o con una licenza
  che ne permetta l'uso.

## Come l'app usa i pacchetti
- Un pacchetto importato è subito **attivo**: le sue voci compaiono nella scheda del personaggio del Master e dei
  giocatori collegati. Si può disattivare (resta salvato) o eliminare.
- Più pacchetti attivi si sommano. Se due pacchetti hanno la stessa voce (stesso `id` per classi, razze e background;
  stesso nome per le altre) vale quella del pacchetto **importato per ultimo**; la pagina lo segnala.
- Lo stesso file non si può importare due volte. Per aggiornare un pacchetto: importa la versione nuova, poi elimina o
  disattiva quella vecchia.
- «⬇ Esporta» restituisce il file originale, per salvarlo o portarlo su un altro PC.

## Intestazione del pacchetto
| Campo | Obbligatorio | Contenuto |
|---|---|---|
| `formato` | sì | sempre `"gasterbless-contenuti"` |
| `versioneFormato` | sì | sempre `1` |
| `sistema` | sì | sistema di gioco. Per ora solo `"d20-5e"` (meccaniche della quinta edizione: d20 + modificatore) |
| `nome` | sì | nome del pacchetto |
| `autore`, `versione`, `licenza`, `descrizione` | no | testi liberi, mostrati nella pagina |
| `fonti` | no | nomi estesi delle fonti: `{"MIO": "Il mio supplemento"}`. Nella scheda, al posto del codice `MIO` compare il nome |
| `listeIncantesimi` | no | liste di incantesimi nuove: `{"l": "Custode"}` (codice di **una lettera minuscola** → nome). Le liste già note alla scheda sono `a` Artefice, `b` Bardo, `c` Chierico, `d` Druido, `p` Paladino, `r` Ranger, `s` Stregone, `w` Warlock, `m` Mago |
| `conosciutiSottoclassiIncantatrici` | no | 20 numeri: incantesimi conosciuti per livello dalle sottoclassi incantatrici (tipo `third`) |
| `descrizioniGenerali` | no | `{"nome del privilegio": "sintesi"}`: spiegazioni valide per qualsiasi classe |
| `descrizioniRisorse` | no | `{"nome della risorsa": "sintesi"}` |
| `classi`, `razze`, `background`, `incantesimi`, `armi`, `armature`, `talenti`, `stili` | almeno uno | elenchi delle voci, descritti sotto |

Codici usati in più punti:
- **caratteristiche**: `FOR`, `DES`, `COS`, `INT`, `SAG`, `CAR`;
- **abilità** (nomi esatti): Acrobazia, Addestrare Animali, Arcano, Atletica, Furtività, Indagare, Inganno, Intimidire,
  Intrattenere, Intuizione, Medicina, Natura, Percezione, Persuasione, Rapidità di Mano, Religione, Sopravvivenza,
  Storia. Un nome diverso si può importare, ma la scheda non lo calcola (l'app lo segnala come avviso);
- **competenze nelle armature**: `l` leggere, `m` medie, `p` pesanti, `sc` scudi; **nelle armi**: `s` semplici, `g` da guerra;
- **id**: lettere minuscole senza accenti, cifre, `-` e `_` (al massimo 40), per esempio `custode`;
- **privilegi**: un oggetto livello → elenco di nomi, `{"1": ["Vigilanza", "Luce Guida"], "3": ["Ordine della Lanterna"]}`;
  i nomi non possono contenere `|` o `;`. Le spiegazioni dei privilegi si scrivono in `descrizioni`.

## Classi (`classi`)
| Campo | Obbligatorio | Contenuto |
|---|---|---|
| `id` | sì | identificativo della classe |
| `nome` | sì | nome mostrato |
| `fonte` | no | codice della fonte |
| `dadoVita` | sì | facce del dado vita (da 4 a 20) |
| `tiriSalvezza` | no | caratteristiche dei tiri salvezza con competenza |
| `armature`, `armi` | no | competenze (codici sopra) |
| `armiSpecifiche` | no | nomi di singole armi con competenza |
| `testoArmature`, `testoArmi`, `strumenti` | no | testi mostrati nella scheda |
| `abilitaDaScegliere`, `abilita` | no | quante abilità sceglie il personaggio e fra quali |
| `lingue` | no | lingue aggiunte dalla classe |
| `livelliAumento` | no | livelli con l'aumento delle caratteristiche (se manca: 4, 8, 12, 16, 19) |
| `stiliDiCombattimento` | no | `1` = uno stile al 1° livello, `2` = uno stile dal 2° livello |
| `incantatore` | no | `{"tipo", "caratteristica", "preparazione", "lista", "conosciuti", "trucchetti"}`: `tipo` `full` (completo), `half` (metà), `pact` (magia del patto), `art` (metà arrotondata per eccesso); `preparazione` `full`, `half` o `known` (incantesimi conosciuti); `lista` codice della lista; `conosciuti` 20 numeri (con `known`); `trucchetti` 3 numeri: trucchetti al 1° livello e i due livelli in cui se ne aggiunge uno |
| `nomeSottoclasse`, `livelloSottoclasse` | no | nome della scelta (es. «Ordine») e livello in cui si sceglie (se manca: 3) |
| `privilegi`, `privilegiOpzionali` | no | privilegi per livello |
| `equipaggiamento` | no | elenco di righe |
| `descrizioni` | no | `{"nome del privilegio": "sintesi"}` |
| `sottoclassi` | no | elenco di sottoclassi (sotto) |

**Sottoclassi**: `nome` (obbligatorio), `fonte`, `privilegi`, `descrizioni` (elenco di spiegazioni nell'ordine dei
privilegi), `armature`, `armi`, `armiSpecifiche`, `abilitaDaScegliere`, `abilitaFisse`, `maestrieExtra` (numero di
maestrie in più), `maestrieFisse`, `attaccoExtra` (`true`), `stiliDiCombattimento`, `incantatore`
(`{"tipo": "third", "caratteristica": "INT"}` per le sottoclassi incantatrici), `agganci` (sotto).

**Calcoli automatici della scheda.** La scheda della quinta edizione ha calcoli propri per alcune classi, riconosciute
dal loro `id`: `artefice`, `barbaro`, `bardo`, `chierico`, `druido`, `guerriero`, `ladro`, `mago`, `monaco`, `paladino`,
`ranger`, `stregone`, `warlock` (risorse per livello, difesa senza armatura, attacchi extra e simili). Una classe con
un altro `id` usa solo i dati del pacchetto.

**Agganci delle sottoclassi** (`"agganci": {"champ": 1}`), per attivare calcoli della scheda:
| Aggancio | Effetto nella scheda |
|---|---|
| `champ` | bonus alle prove fisiche dal 7° livello e stile aggiuntivo dal 10° (guerriero) |
| `bm` | dadi di superiorità e manovre (guerriero) |
| `rune` | numero di rune conosciute (guerriero) |
| `psi` | dadi di energia psionica (guerriero o ladro) |
| `moon` | grado di sfida più alto per le forme animali (druido) |
| `drac` | difesa 13 + Destrezza e punti ferita in più (stregone) |
| `hex` | promemoria per usare il Carisma con l'arma legata (warlock) |
| `initCar`, `initInt`, `initSag` | modificatore di Carisma (dal 3°), Intelligenza (dal 2°) o Saggezza (dal 3°) all'iniziativa |

## Razze (`razze`)
| Campo | Obbligatorio | Contenuto |
|---|---|---|
| `id`, `nome` | sì | identificativo e nome |
| `fonte` | no | codice della fonte |
| `caratteristiche` | no | bonus fissi: `{"DES": 2}` |
| `bonusLiberi` | no | bonus a scelta: `{"numero": 2, "valore": 1, "escluse": ["CAR"]}` |
| `bonusLiberiValori` | no | bonus da assegnare liberamente, es. `[2, 1]` |
| `bonusFissi` | no | `true` = i bonus non si possono riassegnare con la regola facoltativa dell'origine personalizzata |
| `velocita` | no | in metri (se manca: 9) |
| `taglia` | no | testo (se manca: «Media») |
| `scurovisione` | no | in metri |
| `lingue`, `lingueExtra` | no | lingue conosciute e quante sceglierne in più |
| `abilitaFisse`, `abilitaDaScegliere` | no | competenze nelle abilità |
| `tratti` | no | nomi dei tratti; le spiegazioni in `descrizioni` |
| `magiaInnata` | no | `{"nome", "caratteristica", "incantesimi": [{"livello": 1, "testo": "luce (trucchetto)"}]}` |
| `armiSpecifiche`, `armature`, `strumenti` | no | competenze |
| `talento` | no | `true` = un talento al 1° livello |
| `armaturaNaturale` | no | CA base senza armatura (più la Destrezza) |
| `pfPerLivello` | no | punti ferita in più per livello |
| `agganci` | no | `dragon` (scelta dell'antenato e soffio), `dwarf` (l'armatura pesante non riduce la velocità), `tief` (magia innata del retaggio), `lineage` (scurovisione o abilità a scelta); per una sottorazza anche `drow` |
| `descrizioni` | no | `{"nome del tratto": "sintesi"}` |
| `sottorazze` | no | elenco con `id`, `nome` e gli stessi campi della razza (si sommano a quelli della razza) |

## Background (`background`)
`id` e `nome` (obbligatori), `abilita`, `abilitaDaScegliere`, `lingueExtra`, `strumenti`, `privilegio` (nome),
`descrizionePrivilegio`, `equipaggiamento` (testo), `monete` (monete d'oro iniziali).

## Incantesimi (`incantesimi`)
| Campo | Obbligatorio | Contenuto |
|---|---|---|
| `nome` | sì | nome mostrato; `nomeInglese` facoltativo (la scheda riconosce entrambi) |
| `livello` | sì | da 0 (trucchetto) a 9 |
| `scuola` | sì | `abiurazione`, `evocazione`, `divinazione`, `ammaliamento`, `invocazione`, `illusione`, `necromanzia`, `trasmutazione` (oppure la lettera A, C, D, E, V, I, N, T) |
| `tempo` | sì | `azione`, `azione bonus`, `reazione` oppure una durata come `1m`, `10m`, `1h` |
| `rituale` | no | `true` se si può lanciare come rituale |
| `durata` | no | `istantanea`, `permanente`, `speciale` oppure numero e unità (`r` round, `m` minuti, `h` ore, `g` giorni): `1m`, `8h`; con `concentrazione: true` diventa `C10m`. Anche «`I o 1h`» |
| `gittata`, `componenti` | no | testi (es. `18 m`, `V, S, M`) |
| `classi` | no | liste in cui compare: `"bcm"` oppure `["b", "c", "m"]` |
| `fonte`, `descrizione`, `livelliSuperiori` | no | testi |

La scheda legge dalla descrizione attacchi, tiri salvezza, danni e cure per l'automazione nello scontro, quando li
riconosce; altrimenti decide il Master.

## Armi, armature, talenti, stili
- **Armi** (`armi`): `nome`, `categoria` (`semplice` o `guerra`), `tipo` (`mischia` o `distanza`), `danno` (`1d8`, `2d6`
  o un numero), `tipoDanno`, `proprieta` (testo mostrato) e, per i calcoli, `accurata`, `leggera`, `pesante`, `dueMani`,
  `lancio`, `versatile`, `portata` (`true`).
- **Armature** (`armature`): `nome`, `tipo` (`leggera`, `media`, `pesante`), `ca`, `forza` (Forza minima, 0 se nessuna),
  `svantaggioFurtivita` (`true`/`false`).
- **Talenti** (`talenti`): `nome` (obbligatorio), `nomeInglese`, `fonte`, `caratteristiche` (`null` = nessun +1, `"*"` =
  qualsiasi, oppure un elenco come `["FOR", "DES"]`), `descrizione`, `effetti` calcolati dalla scheda: `init` (bonus
  all'iniziativa), `speed` (metri), `hpLvl` (PF per livello), `pass` (Percezione passiva), `saveAb` (`1`: competenza nel
  TS della caratteristica scelta), `armor` (competenze, es. `["l", "m"]`), `medDex3` (`1`), `unarmored` (`1`: CA 13 +
  Destrezza), `res` (`["nome", valore]` di una risorsa).
- **Stili di combattimento** (`stili`): `nome`, `descrizione`, `opzionale` (`true` per gli stili della regola facoltativa).

## Modifiche future del formato
Una versione nuova del formato avrà un numero nuovo in `versioneFormato`; l'app continuerà a leggere i pacchetti della
versione 1. Altri sistemi di gioco avranno un valore diverso in `sistema` e campi propri.

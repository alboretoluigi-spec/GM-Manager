package server

// "Che sfortuna": quando un giocatore ottiene un 1 naturale sul d20 durante uno scontro, sul suo
// schermo compare un pulsante. Premendolo, il server tira un d100 che nessuno vede e sceglie un
// effetto dalla tabella; l'effetto (non il numero né la tabella) compare sugli schermi della sessione.
// La tabella predefinita è scritta per Gaster Bless; il Master può sostituirla con la sua.

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"gasterbless/internal/cripto"
)

// Tabella predefinita: 100 disavventure scritte per l'app (una per risultato del d100).
var sfortunaPredefinita = []string{
	"L'arma ti scivola di mano e cade a 1,5 m da te, in una direzione casuale.",
	"Inciampi nei tuoi stessi piedi e cadi prono.",
	"Ti colpisci da solo: subisci metà dei danni che avresti inflitto.",
	"La corda dell'arco (o la cinghia dello scudo) si allenta: ti serve un'azione per sistemarla.",
	"Ti entra qualcosa in un occhio: fino alla fine del tuo prossimo turno hai svantaggio ai tiri per colpire.",
	"Urli una sfida al nemico ma ti esce un acuto ridicolo: il nemico ha vantaggio al prossimo attacco contro di te.",
	"Calpesti il mantello di un alleato: lui deve superare un TS su Destrezza CD 10 o cadere prono.",
	"La tua arma si incastra nel terreno o in un mobile: serve un'azione e una prova di Forza CD 12 per liberarla.",
	"Perdi l'equilibrio: la tua velocità è dimezzata fino alla fine del tuo prossimo turno.",
	"Un fischio nelle orecchie ti distrae: non puoi effettuare reazioni fino al tuo prossimo turno.",
	"Ti si slaccia uno stivale: il tuo prossimo movimento costa il doppio.",
	"Lo slancio ti porta troppo avanti: ti sposti di 1,5 m verso il nemico, provocando un attacco di opportunità se possibile.",
	"Colpisci un alleato vicino al posto del nemico (tira i danni normalmente).",
	"Il sudore ti fa scivolare la presa: alla fine del turno l'arma cade ai tuoi piedi.",
	"Un crampo alla gamba: sei trattenuto fino all'inizio del tuo prossimo turno.",
	"Ti mordi la lingua: non puoi lanciare incantesimi con componenti verbali fino al tuo prossimo turno.",
	"Il tuo zaino si apre e un oggetto a caso rotola a terra.",
	"Ti volti al rumore sbagliato: il primo attacco contro di te prima del tuo prossimo turno ha vantaggio.",
	"La tua arma si scheggia: −1 ai danni con essa fino al prossimo riposo lungo.",
	"Una manciata di monete ti cade dalla borsa: perdi 1d10 monete d'oro (o d'argento, se sei povero).",
	"Fai un passo indietro e finisci in una pozza: sei rallentato (velocità −3 m) fino al tuo prossimo turno.",
	"Il colpo va così a vuoto che ruoti su te stesso: concedi vantaggio al prossimo attacco contro di te.",
	"Il tuo simbolo sacro, focus o strumento cade a terra.",
	"Ti si impiglia l'arma nell'armatura: −2 alla CA fino all'inizio del tuo prossimo turno.",
	"Sbagli così platealmente che un alleato ride: lui ha svantaggio al prossimo tiro per colpire.",
	"Ti accorgi di avere una sanguisuga attaccata: subisci 1 danno all'inizio dei tuoi prossimi 3 turni.",
	"Starnutisci forte: chiunque fosse nascosto vicino a te sa dove sei, e tu perdi la concentrazione.",
	"Ti si allenta l'elmo e ti scivola sugli occhi: sei accecato fino alla fine del tuo prossimo turno.",
	"Perdi l'iniziativa: nel prossimo round agisci per ultimo.",
	"Un sasso sotto il piede ti fa ruzzolare 1,5 m in una direzione casuale.",
	"Il tuo grido di battaglia ti strozza: hai svantaggio alle prove di Carisma fino alla fine dello scontro.",
	"L'arma ti rimbalza contro: subisci 1d4 danni contundenti.",
	"Ti si apre una cucitura dei pantaloni: hai svantaggio alle prove di Furtività fino al prossimo riposo.",
	"Ti ferisci la mano: −1 ai tiri per colpire fino al prossimo riposo breve.",
	"Un pipistrello (o un corvo di Gaster Bless) ti vola in faccia: perdi la prossima reazione.",
	"Colpisci un oggetto vicino e lo rompi (una sedia, una lanterna, una bottiglia: decide il Master).",
	"La tua munizione o la tua arma da lancio vola lontanissimo e si perde.",
	"Ti viene il singhiozzo: ogni incantesimo con componente verbale richiede una prova di Costituzione CD 10.",
	"Il nemico approfitta dell'errore: può muoversi di 3 m senza provocare attacchi di opportunità.",
	"Ti appoggi a una parete che non c'è: cadi prono.",
	"Lasci cadere un'arma da mischia e un'arma secondaria insieme.",
	"Ti accorgi di avere la borraccia bucata: hai perso l'acqua per oggi.",
	"Il terreno sotto di te diventa difficile (fango, olio, sangue) in un quadrato di 1,5 m.",
	"Ti si annebbia la vista per un attimo: −2 alla Percezione passiva fino alla fine dello scontro.",
	"Colpisci l'aria con tanta forza che ti stiri un muscolo: svantaggio alle prove di Forza fino al prossimo riposo breve.",
	"Un alleato a scelta del Master deve usare la reazione per afferrarti prima che tu cada.",
	"Ti senti osservato dalle nebbie di Gaster Bless: hai svantaggio al prossimo tiro salvezza su Saggezza.",
	"Il tuo mantello prende fuoco da una torcia vicina: 1d4 danni da fuoco, poi lo spegni.",
	"Fai rumore di ferraglia: tutte le creature entro 18 m sanno dove sei.",
	"Ti sfugge una parola di troppo: rivela al nemico un piccolo segreto del gruppo (lo sceglie il Master).",
	"L'arma ti sfugge e vola 3 m in una direzione casuale.",
	"Scivoli in avanti e finisci prono ai piedi del nemico.",
	"Il colpo rimbalza e ferisce l'alleato più vicino a te (metà dei danni); se non ce ne sono entro 1,5 m, nessun effetto.",
	"Ti si stacca una cinghia dell'armatura: −1 alla CA fino a quando non spendi 1 minuto per sistemarla.",
	"Sei così imbarazzato che non riesci ad aiutare: non puoi usare l'azione Aiuto fino alla fine dello scontro.",
	"Ti cade il cappuccio o il cappello e ti copre la faccia: svantaggio al prossimo attacco.",
	"Un insetto ti entra nell'armatura: sei distratto, −2 ai TS su Destrezza fino al tuo prossimo turno.",
	"La tua ombra sembra muoversi da sola per un istante: sei spaventato dal nemico più vicino fino al tuo prossimo turno.",
	"Tiri così male che l'arma resta piantata in un nemico: per riprenderla serve un'azione.",
	"Perdi il conto delle frecce o delle munizioni: ne hai 1d4 in meno.",
	"Ti si apre una pozione nella borsa: perdila (se ne hai una), altrimenti sei bagnato e appiccicoso.",
	"Fai un passo falso verso il nemico: entri nella sua portata e lui può attaccarti con la reazione.",
	"Un alleato distratto dal tuo errore perde metà del suo prossimo movimento.",
	"Il vento (o un soffio misterioso) ti spinge di 1,5 m indietro.",
	"La tua arma si piega: diventa un'arma improvvisata fino a quando non viene riparata.",
	"Ti torci la caviglia: velocità −3 m fino al prossimo riposo lungo.",
	"Batti la testa contro qualcosa: sei stordito fino alla fine del tuo turno.",
	"Gridi il nome sbagliato del tuo dio, del tuo patrono o del tuo maestro: nessun effetto, ma tutti ti hanno sentito.",
	"La tua armatura cigola orribilmente: svantaggio a Furtività fino al prossimo riposo lungo.",
	"Ti accorgi di aver lasciato la porta (o il cancello) aperta alle tue spalle: qualcosa potrebbe seguirvi.",
	"Il colpo mancato ti fa girare la testa: hai svantaggio al prossimo tiro salvezza su Costituzione.",
	"Lanci l'arma per errore: cade a 1d6 × 1,5 m davanti a te.",
	"Finisci addosso a un alleato: entrambi dovete superare un TS su Forza CD 10 o cadere proni.",
	"La paura ti blocca un istante: perdi l'azione bonus del prossimo turno.",
	"Una goccia di cera o di pioggia ti acceca un occhio: −1 ai tiri per colpire a distanza fino al prossimo riposo breve.",
	"Ti strappi la manica: si vede un tatuaggio, un segno o un ricordo che preferivi nascondere.",
	"Lo scudo ti scivola sul braccio sbagliato: non ottieni il bonus dello scudo fino al tuo prossimo turno.",
	"Ti si incrinano le lenti, gli occhiali o il monocolo (se ne hai): svantaggio alle prove di Indagare.",
	"Un topo ti morde la caviglia: 1 danno perforante e un grido poco eroico.",
	"Il tuo animale, famiglio o cavalcatura si spaventa e si allontana di 9 m.",
	"Sbagli il tempo: il tuo prossimo attacco avviene dopo quello di tutti gli altri nel round.",
	"Ti cade a terra la torcia o la fonte di luce: l'area intorno a te diventa in penombra.",
	"L'arma ti si rivolta contro: tira di nuovo l'attacco contro la tua CA; se colpisci, subisci i danni.",
	"Tremi per il freddo di Gaster Bless: −1 ai TS su Costituzione fino alla fine dello scontro.",
	"Ti si inceppa la balestra o l'arma meccanica: serve un'azione per sbloccarla.",
	"Ti colpisce un ricordo doloroso: hai svantaggio alla prossima prova di caratteristica.",
	"Le tue parole d'incoraggiamento demoralizzano il gruppo: gli alleati entro 3 m hanno −1 al prossimo tiro.",
	"La foga ti fa perdere il fiato: ottieni un livello di indebolimento fino alla fine dello scontro.",
	"Un oggetto magico che porti emette una luce o un suono per 1 minuto, rivelando la tua posizione.",
	"Rompi una fiala, un componente o una pergamena a caso tra quelle che porti.",
	"Calpesti una trappola dimenticata: TS su Destrezza CD 12 o 1d6 danni perforanti.",
	"Ti ritrovi con la faccia nel fango: hai svantaggio alle prove di Carisma (Persuasione) fino a quando non ti lavi.",
	"Il nemico ti disarma con un gesto beffardo: l'arma ora è ai suoi piedi.",
	"Vedi per un attimo un volto pallido nella nebbia: sei spaventato fino alla fine del tuo prossimo turno.",
	"Ti accorgi che la tua borsa è più leggera: un ladruncolo (o un corvo) ti ha preso un oggetto di poco valore.",
	"Ti si spezza un laccio della borsa dei componenti: il prossimo incantesimo con componenti materiali richiede un'azione in più per prepararli.",
	"Fai così tanta confusione che il nemico più vicino ha vantaggio al prossimo TS contro di te.",
	"Il tuo stesso grido ti fa sobbalzare: perdi 1d4 punti ferita temporanei (se ne hai) o 1 punto ferita.",
	"Un brivido ti attraversa: il Master può far ritirare un tuo tiro riuscito entro il prossimo minuto.",
	"La sfortuna è totale: tira di nuovo su questa tabella e applica entrambi gli effetti (il Master decide se è troppo).",
}

const chiaveSfortuna = "sfortuna_elenco"

// tabellaSfortuna: quella del Master se l'ha impostata, altrimenti la predefinita.
func (s *Server) tabellaSfortuna() (l []string, personalizzata bool) {
	if v, err := s.impostazione(chiaveSfortuna); err == nil && strings.TrimSpace(v) != "" {
		for _, r := range strings.Split(v, "\n") {
			if r = strings.TrimSpace(r); r != "" {
				l = append(l, r)
			}
		}
		if len(l) > 0 {
			return l, true
		}
	}
	return sfortunaPredefinita, false
}

// sfortunaInAttesa: i 1 naturali in scontro che possono ancora usare il pulsante (uno per tiro, 10 minuti).
type attesaSfortuna struct {
	utente, sessione, chi string
	scade                 time.Time
}

var (
	muSfortuna       sync.Mutex
	sfortunaInAttesa = map[string]attesaSfortuna{}
)

func registraSfortuna(id, utente, sessione, chi string) {
	muSfortuna.Lock()
	defer muSfortuna.Unlock()
	ora := time.Now()
	for k, v := range sfortunaInAttesa {
		if ora.After(v.scade) {
			delete(sfortunaInAttesa, k)
		}
	}
	sfortunaInAttesa[id] = attesaSfortuna{utente, sessione, chi, ora.Add(10 * time.Minute)}
}

func (s *Server) rotteSfortuna(mux *http.ServeMux) {
	s.Hub.Registra("dadi.sfortuna", s.wsSfortuna)
	// Il Master legge e sostituisce la tabella (ai giocatori non viene mai mandata).
	mux.Handle("GET /api/v1/sfortuna", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := richiedi(r, "master"); err != nil {
			return nil, err
		}
		l, pers := s.tabellaSfortuna()
		return map[string]any{"effetti": l, "personalizzata": pers, "predefinita": len(sfortunaPredefinita)}, nil
	}))
	mux.Handle("PUT /api/v1/sfortuna", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := richiedi(r, "master"); err != nil {
			return nil, err
		}
		var in struct {
			Effetti []string `json:"effetti"`
		}
		if err := leggiJSON(r, &in); err != nil {
			return nil, err
		}
		var l []string
		for _, e := range in.Effetti {
			if e = strings.TrimSpace(strings.ReplaceAll(e, "\n", " ")); e != "" {
				if len([]rune(e)) > 300 {
					return nil, errore(http.StatusBadRequest, "Ogni effetto può avere al massimo 300 caratteri.")
				}
				l = append(l, e)
			}
		}
		if len(l) > 100 {
			return nil, errore(http.StatusBadRequest, "Al massimo 100 effetti (uno per risultato del d100).")
		}
		// Elenco vuoto = torna la tabella predefinita.
		return nil, s.impostaImpostazione(chiaveSfortuna, strings.Join(l, "\n"))
	}))
}

// wsSfortuna: il giocatore preme "Che sfortuna" dopo il suo 1 naturale. Il d100 resta nascosto.
func (s *Server) wsSfortuna(c *Cliente, d json.RawMessage) (any, error) {
	var in struct {
		Tiro string `json:"tiro"`
	}
	if json.Unmarshal(d, &in) != nil || in.Tiro == "" {
		return nil, errDatiNonValidi
	}
	muSfortuna.Lock()
	a, ok := sfortunaInAttesa[in.Tiro]
	if ok && a.utente == c.U.ID && time.Now().Before(a.scade) {
		delete(sfortunaInAttesa, in.Tiro) // si usa una volta sola
	} else {
		ok = false
	}
	muSfortuna.Unlock()
	if !ok {
		return nil, errore(http.StatusConflict, "Questa sfortuna è già stata tirata (o è scaduta).")
	}
	l, _ := s.tabellaSfortuna()
	d100 := tiraDado(100)
	effetto := l[(d100-1)*len(l)/100]
	ris := map[string]any{"id": cripto.ID(), "sessione": a.sessione, "chi": a.chi, "chiId": c.U.ID, "effetto": effetto}
	for _, id := range s.destinatariSessione(a.sessione) {
		s.Hub.AUtente(id, Msg{Tipo: "dadi.sfortuna", Dati: ris})
	}
	if comb, _, _, _, attivo := s.scontroAttivo(a.sessione); attivo {
		s.registro(comb, c.U, "che sfortuna! "+effetto)
	}
	return ris, nil
}

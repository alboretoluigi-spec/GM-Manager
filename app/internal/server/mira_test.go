package server

import (
	"strings"
	"testing"
)

// «X prende di mira Y» (2026-11-12): il Master sceglie il bersaglio di un mostro e tutti i presenti lo vedono; il PG
// preso di mira è segnato per il suo giocatore; un mostro nascosto non si rivela; i giocatori non possono mandarlo.
func TestPrendeDiMira(t *testing.T) {
	sc := nuovoScontro(t)
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "l'antagonista", "quanti": 1, "ca": 16, "pf": 144})
	vm := sc.vista(sc.m)
	idA, idS := cerca(vm, "Arin")["id"].(string), cerca(vm, "l'antagonista")["id"].(string)
	cl, cm := sc.clienteWS(sc.luca), sc.clienteWS(sc.m)
	ultimiMessaggi(cl, "")
	ultimiMessaggi(cm, "")
	sc.az(sc.m, 200, "iniz.mira", map[string]any{"combattente": idS, "bersagli": []string{idA}})
	ml := ultimiMessaggi(cl, "iniz.mira")
	if len(ml) != 1 || !strings.Contains(ml[0], `"chi":"l'antagonista"`) || !strings.Contains(ml[0], `"bersagli":["Arin"]`) || !strings.Contains(ml[0], sc.idLuca) {
		t.Fatalf("messaggio al giocatore: %v", ml)
	}
	if len(ultimiMessaggi(cm, "iniz.mira")) != 1 {
		t.Fatal("anche il Master vede il messaggio")
	}
	// il giocatore non può: il suo PG non è un mostro e non è il Master
	sc.az(sc.luca, 403, "iniz.mira", map[string]any{"combattente": idS, "bersagli": []string{idA}})
	sc.az(sc.m, 403, "iniz.mira", map[string]any{"combattente": idA, "bersagli": []string{idS}})
	sc.az(sc.m, 400, "iniz.mira", map[string]any{"combattente": idS, "bersagli": []string{"nessuno"}})
	// mostro nascosto: niente messaggio
	sc.az(sc.m, 200, "iniz.nascondi", map[string]any{"combattente": idS, "nascosto": true})
	ultimiMessaggi(cl, "")
	sc.az(sc.m, 200, "iniz.mira", map[string]any{"combattente": idS, "bersaglio": idA})
	if ml := ultimiMessaggi(cl, "iniz.mira"); len(ml) != 0 {
		t.Fatalf("un mostro nascosto si è rivelato: %v", ml)
	}
}

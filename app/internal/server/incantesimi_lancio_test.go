package server

import (
	"strings"
	"testing"
)

// Lancio di un incantesimo dalla scheda: lo vedono il Master e il gruppo del personaggio; va nel registro.
func TestLancioIncantesimo(t *testing.T) {
	sc := nuovoScontro(t)
	cm, cl, cs, cu := sc.clienteWS(sc.m), sc.clienteWS(sc.luca), sc.clienteWS(sc.sara), sc.clienteWS(sc.ugo)
	for _, c := range []*Cliente{cm, cl, cs, cu} {
		ultimiMessaggi(c, "")
	}
	inc := map[string]any{"nome": "Palla di fuoco", "en": "Fireball", "livello": 3, "slot": 3, "descrizione": "Un lampo di fuoco esplode."}
	if _, err := sc.ws(cu, "incantesimo.lancia", map[string]any{"personaggio": sc.pg, "inc": inc}); err == nil {
		t.Fatal("Ugo non può lanciare per il personaggio di Luca")
	}
	sc.wsOK(cl, "incantesimo.lancia", map[string]any{"personaggio": sc.pg, "inc": inc})
	for nome, c := range map[string]*Cliente{"Master": cm, "Luca": cl, "Sara": cs} {
		if m := ultimiMessaggi(c, "incantesimo.lanciato"); len(m) != 1 || !strings.Contains(m[0], "Un lampo di fuoco") {
			t.Fatalf("%s deve vedere la descrizione: %v", nome, m)
		}
	}
	if len(ultimiMessaggi(cu, "incantesimo.lanciato")) != 0 {
		t.Fatal("chi è in un altro gruppo non la vede")
	}
	v := sc.a.s.vistaCombattimento(sc.ses, cm.U)
	if !strings.Contains(strings.Join(v.Registro, "|"), "Arin lancia Palla di fuoco (slot di 3° livello)") {
		t.Fatalf("il lancio va nel registro dello scontro: %v", v.Registro)
	}
	if _, err := sc.ws(cl, "incantesimo.lancia", map[string]any{"personaggio": sc.pg, "inc": map[string]any{"nome": "", "livello": 1}}); err == nil {
		t.Fatal("dati non validi rifiutati")
	}
}

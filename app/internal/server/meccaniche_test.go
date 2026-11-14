package server

import (
	"strings"
	"testing"
)

// dadiFissi: i prossimi risultati dei dadi, in ordine (per provare gli esiti delle meccaniche).
func dadiFissi(t *testing.T, v ...int) {
	t.Helper()
	prima := tiraDado
	coda := append([]int{}, v...)
	tiraDado = func(f int) int {
		if len(coda) == 0 {
			t.Fatalf("dadi finiti (d%d)", f)
		}
		x := coda[0]
		coda = coda[1:]
		return x
	}
	t.Cleanup(func() { tiraDado = prima })
}

// Meccaniche delle famiglie (fase 3c, C4c): esiti con dadi noti, limiti e errori.
func TestMeccaniche(t *testing.T) {
	d := func(n int) *int { return &n }
	casi := []struct {
		nome  string
		dadi  []int
		in    richiestaMeccanica
		esito string
		tot   int
		testo string
	}{
		{"d100 estremo", []int{3, 1}, richiestaMeccanica{Meccanica: "d100", Valore: 60}, "estremo", 2, "Successo estremo"},
		{"d100 critico", []int{2, 1}, richiestaMeccanica{Meccanica: "d100", Valore: 60}, "critico", 1, "Successo critico"},
		{"d100 difficile", []int{6, 3}, richiestaMeccanica{Meccanica: "d100", Valore: 60}, "difficile", 25, "Successo difficile"},
		{"d100 fallimento", []int{1, 8}, richiestaMeccanica{Meccanica: "d100", Valore: 60}, "fallimento", 70, "Fallimento"},
		{"d100 disastro con valore basso", []int{8, 10}, richiestaMeccanica{Meccanica: "d100", Valore: 40}, "disastro", 97, "Disastro"},
		{"d100 100", []int{1, 1}, richiestaMeccanica{Meccanica: "d100", Valore: 90}, "disastro", 100, "Disastro"},
		{"d100 dado bonus", []int{6, 8, 2}, richiestaMeccanica{Meccanica: "d100", Valore: 60, Extra: 1}, "difficile", 15, "Successo difficile"},
		{"d100 dado penalità", []int{6, 8, 2}, richiestaMeccanica{Meccanica: "d100", Valore: 60, Extra: -1}, "fallimento", 75, "Fallimento"},
		{"riserva d10 coppie di 10", []int{10, 10, 7, 3, 1}, richiestaMeccanica{Meccanica: "riserva-d10", Dadi: 5, Difficolta: d(2), Opzioni: map[string]any{"coppieDi10": true}}, "critico", 5, "Successo critico (5 successi)"},
		{"riserva d10 fallimento", []int{2, 5, 3}, richiestaMeccanica{Meccanica: "riserva-d10", Dadi: 3}, "fallimento", 0, "Fallimento (0 successi su 1)"},
		{"riserva d6 successi da 5", []int{5, 6, 2, 4}, richiestaMeccanica{Meccanica: "riserva-d6", Dadi: 3, Mod: 1, Soglia: 5, Difficolta: d(2)}, "successo", 2, "Successo (2 successi)"},
		{"riserva d6 disastro con 1", []int{1, 3}, richiestaMeccanica{Meccanica: "riserva-d6", Dadi: 2, Opzioni: map[string]any{"unoDisastro": true}}, "disastro", 0, "Disastro"},
		{"riserva d6 massimo critico", []int{6, 2, 6}, richiestaMeccanica{Meccanica: "riserva-d6", Dadi: 3, Modo: "massimo"}, "critico", 6, "Critico"},
		{"riserva d6 massimo parziale", []int{5, 3}, richiestaMeccanica{Meccanica: "riserva-d6", Dadi: 2, Modo: "massimo"}, "parziale", 5, "Successo parziale"},
		{"riserva d6 senza dadi", []int{6, 2}, richiestaMeccanica{Meccanica: "riserva-d6", Dadi: 0, Modo: "massimo"}, "fallimento", 2, "Fallimento"},
		{"2d6 pieno", []int{4, 5}, richiestaMeccanica{Meccanica: "2d6-fasce", Mod: 1}, "successo", 10, "Successo pieno"},
		{"2d6 parziale", []int{3, 4}, richiestaMeccanica{Meccanica: "2d6-fasce"}, "parziale", 7, "Successo parziale"},
		{"2d6 fallimento", []int{1, 2}, richiestaMeccanica{Meccanica: "2d6-fasce", Mod: 2}, "fallimento", 5, "Fallimento"},
		{"d10 + valore critico", []int{10, 4}, richiestaMeccanica{Meccanica: "d10-somma", Valore: 8}, "critico", 22, "Successo critico"},
		{"d10 + valore fallimento critico", []int{1, 5}, richiestaMeccanica{Meccanica: "d10-somma", Valore: 8}, "disastro", 4, "Fallimento critico"},
		{"d10 + valore successo", []int{6}, richiestaMeccanica{Meccanica: "d10-somma", Valore: 9, Difficolta: d(15)}, "successo", 15, "Successo"},
		{"passi con incremento", []int{8, 3}, richiestaMeccanica{Meccanica: "passi-esplosivi", Dadi: 8}, "critico", 11, "Successo con 1 incremento"},
		{"passi disastro con il jolly", []int{1, 1}, richiestaMeccanica{Meccanica: "passi-esplosivi", Dadi: 6, Opzioni: map[string]any{"jolly": true}}, "disastro", 1, "Disastro"},
		{"passi jolly migliore", []int{2, 5}, richiestaMeccanica{Meccanica: "passi-esplosivi", Dadi: 4, Opzioni: map[string]any{"jolly": true}}, "successo", 5, "Successo"},
		{"fate con stile", []int{1, 3, 5, 6}, richiestaMeccanica{Meccanica: "fate", Valore: 2}, "critico", 3, "Successo con stile (+3)"},
		{"fate pareggio", []int{1, 2, 5, 3}, richiestaMeccanica{Meccanica: "fate", Valore: 1, Difficolta: d(0)}, "parziale", 0, "Pareggio"},
		{"3d6 critico", []int{1, 1, 2}, richiestaMeccanica{Meccanica: "3d6-sotto", Valore: 12}, "critico", 4, "Successo critico"},
		{"3d6 disastro", []int{6, 6, 6}, richiestaMeccanica{Meccanica: "3d6-sotto", Valore: 16}, "disastro", 18, "Fallimento critico"},
		{"3d6 margine", []int{3, 4, 5}, richiestaMeccanica{Meccanica: "3d6-sotto", Valore: 13}, "successo", 12, "Successo (margine 1)"},
		{"d20 sotto critico", []int{1}, richiestaMeccanica{Meccanica: "d20-sotto", Valore: 10}, "critico", 1, "Successo critico (1)"},
		{"d20 sotto disastro", []int{20}, richiestaMeccanica{Meccanica: "d20-sotto", Valore: 10}, "disastro", 20, "Fallimento critico (20)"},
		{"d20 sotto vantaggio", []int{15, 4}, richiestaMeccanica{Meccanica: "d20-sotto", Valore: 10, Modo: "vantaggio"}, "successo", 4, "Successo"},
		{"d20 sotto svantaggio", []int{4, 15}, richiestaMeccanica{Meccanica: "d20-sotto", Valore: 10, Modo: "svantaggio"}, "fallimento", 15, "Fallimento"},
		{"narrativi", []int{4, 8, 2}, richiestaMeccanica{Meccanica: "narrativi", Narrativi: map[string]int{"abilita": 2, "difficolta": 1}}, "successo", 1, "Successo (1 netti) · 2 vantaggi"},
		{"narrativi trionfo", []int{12}, richiestaMeccanica{Meccanica: "narrativi", Narrativi: map[string]int{"competenza": 1}}, "critico", 1, "1 trionfo"},
	}
	for _, c := range casi {
		t.Run(c.nome, func(t *testing.T) {
			dadiFissi(t, c.dadi...)
			x, e, err := tiraMeccanica(c.in)
			if err != nil {
				t.Fatal(err)
			}
			if e.Esito != c.esito || x.Totale != c.tot || !strings.Contains(e.Testo, c.testo) {
				t.Fatalf("esito %s, totale %d, testo «%s» (formula %s)", e.Esito, x.Totale, e.Testo, x.Formula)
			}
			if (c.esito == "critico") != x.Critico || (c.esito == "disastro") != x.Fallimento || len(x.Dadi) == 0 {
				t.Fatalf("segni del tiro: %+v", x)
			}
		})
	}
	for _, in := range []richiestaMeccanica{{Meccanica: "boh"}, {Meccanica: "d100", Valore: 0}, {Meccanica: "d100", Valore: 50, Extra: 3},
		{Meccanica: "riserva-d10", Dadi: 0}, {Meccanica: "riserva-d6", Dadi: 50}, {Meccanica: "riserva-d10", Dadi: 3, Soglia: 11},
		{Meccanica: "passi-esplosivi", Dadi: 7}, {Meccanica: "3d6-sotto", Valore: 0}, {Meccanica: "d20-sotto", Valore: 40},
		{Meccanica: "narrativi"}, {Meccanica: "narrativi", Narrativi: map[string]int{"magia": 1}}, {Meccanica: "2d6-fasce", Mod: 99}} {
		if _, _, err := tiraMeccanica(in); err == nil {
			t.Errorf("%+v doveva essere rifiutato", in)
		}
	}
}

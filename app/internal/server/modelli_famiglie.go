package server

import "strings"

// Modelli di base della scheda libera per ogni famiglia di meccaniche (fase 3c, passo C4d, docs/SCHEDA_LIBERA.md).
// Sono punti di partenza generici, da adattare al gioco nella pagina Campagne: campi con nomi comuni, tiri con la
// meccanica della famiglia (docs/MECCANICHE.md), punti ferita, difesa, iniziativa e valute. Nessun elenco ricalcato da
// un gioco preciso e nessun testo dei manuali.

func pInt(n int) *int { return &n }

// idCampo: identificativo di un campo del modello dal nome («Pronto soccorso» → «pronto_soccorso»).
func idCampo(nome string) string { return strings.ReplaceAll(idDaNome(nome), "-", "_") }

func cNum(id, nome string, min, max, pred int, meccanica bool, aiuto string) campoModello {
	return campoModello{ID: id, Nome: nome, Tipo: "numero", Min: pInt(min), Max: pInt(max), Predefinito: pred, Meccanica: meccanica, Aiuto: aiuto}
}
func cRis(id, nome string, pred int) campoModello {
	return campoModello{ID: id, Nome: nome, Tipo: "risorsa", Predefinito: pred}
}
func cTesto(id, nome string) campoModello { return campoModello{ID: id, Nome: nome, Tipo: "testo"} }
func cArea(id, nome string) campoModello  { return campoModello{ID: id, Nome: nome, Tipo: "area"} }
func cElenco(id, nome string) campoModello {
	return campoModello{ID: id, Nome: nome, Tipo: "elenco"}
}

// valori: campi numerici con lo stesso intervallo e la meccanica del gioco.
func valori(min, max, pred int, aiuto string, nomi ...string) []campoModello {
	out := []campoModello{}
	for _, n := range nomi {
		out = append(out, cNum(idCampo(n), n, min, max, pred, true, aiuto))
	}
	return out
}

func sezPersonaggio() sezioneModello {
	return sezioneModello{Titolo: "Personaggio", Campi: []campoModello{cTesto("concetto", "Concetto"), cArea("aspetto", "Aspetto e storia")}}
}
func sezEquip(valuta string) sezioneModello {
	return sezioneModello{Titolo: "Equipaggiamento e note", Campi: []campoModello{cElenco("equipaggiamento", "Equipaggiamento"),
		cNum(idCampo(valuta), valuta, 0, 9999999, 0, false, "Si gestisce anche dalla sezione Finanze"), cArea("note", "Note")}}
}

// modelloBasePer: il modello di partenza per il gioco della campagna (vuoto = scheda libera generica).
func modelloBasePer(modulo string) modelloScheda {
	switch modulo {
	case "d100":
		return modelloScheda{Versione: 1, Sezioni: []sezioneModello{sezPersonaggio(),
			{Titolo: "Caratteristiche", Campi: valori(1, 100, 50, "", "Forza", "Destrezza", "Costituzione", "Intelligenza", "Volontà", "Fascino")},
			{Titolo: "Abilità", Campi: valori(0, 100, 25, "Percentuale", "Atletica", "Combattimento", "Furtività", "Percezione", "Persuasione", "Conoscenze", "Pronto soccorso")},
			{Titolo: "Condizione", Campi: []campoModello{cRis("punti_ferita", "Punti ferita", 10), cRis("equilibrio", "Equilibrio mentale", 50), cNum("fortuna", "Fortuna", 0, 100, 50, true, "")}},
			sezEquip("Denaro")}, PF: "punti_ferita", Iniziativa: "{destrezza}", Valute: []string{"denaro"}}
	case "riserva-d10":
		return modelloScheda{Versione: 1, Sezioni: []sezioneModello{sezPersonaggio(),
			{Titolo: "Attributi", Campi: valori(1, 5, 2, "Dadi della riserva", "Forza", "Agilità", "Resistenza", "Carisma", "Astuzia", "Intelligenza", "Volontà")},
			{Titolo: "Abilità", Campi: valori(0, 5, 0, "Dadi da aggiungere all'attributo", "Combattimento", "Furtività", "Percezione", "Persuasione", "Conoscenze", "Guida")},
			{Titolo: "Condizione", Campi: []campoModello{cRis("salute", "Salute", 7), cRis("forza_di_volonta", "Forza di volontà", 5)}},
			sezEquip("Denaro")},
			Tiri: []tiroModello{{Nome: "Agilità + Combattimento", Valore: "{agilita} + {combattimento}"}, {Nome: "Astuzia + Percezione", Valore: "{astuzia} + {percezione}"}},
			PF:   "salute", Iniziativa: "1d10 + {agilita} + {astuzia}", Valute: []string{"denaro"}}
	case "riserva-d6":
		return modelloScheda{Versione: 1, Sezioni: []sezioneModello{sezPersonaggio(),
			{Titolo: "Attributi", Campi: valori(1, 5, 2, "Dadi della riserva", "Fisico", "Agilità", "Mente", "Presenza")},
			{Titolo: "Abilità", Campi: valori(0, 5, 0, "Dadi da aggiungere all'attributo", "Forza bruta", "Combattimento", "Furtività", "Osservazione", "Conoscenze", "Persuasione")},
			{Titolo: "Condizione", Campi: []campoModello{cRis("salute", "Salute", 4), cRis("stress", "Stress", 0), cElenco("condizioni", "Condizioni")}},
			sezEquip("Denaro")},
			Tiri: []tiroModello{{Nome: "Fisico + Forza bruta", Valore: "{fisico} + {forza_bruta}"}, {Nome: "Mente + Osservazione", Valore: "{mente} + {osservazione}"}},
			PF:   "salute", Iniziativa: "1d10", Valute: []string{"denaro"}}
	case "2d6-fasce":
		return modelloScheda{Versione: 1, Sezioni: []sezioneModello{sezPersonaggio(),
			{Titolo: "Statistiche", Campi: valori(-2, 3, 0, "Modificatore del 2d6", "Forza", "Agilità", "Ingegno", "Fascino", "Istinto")},
			{Titolo: "Mosse", Campi: []campoModello{cElenco("mosse", "Mosse del personaggio"), cArea("legami", "Legami")}},
			{Titolo: "Condizione", Campi: []campoModello{cRis("danni", "Salute", 6), cNum("esperienza", "Esperienza", 0, 99, 0, false, ""), cElenco("condizioni", "Condizioni")}},
			sezEquip("Denaro")}, PF: "danni", Valute: []string{"denaro"}}
	case "d10-somma":
		return modelloScheda{Versione: 1, Sezioni: []sezioneModello{sezPersonaggio(),
			{Titolo: "Caratteristiche", Campi: valori(1, 10, 5, "", "Corpo", "Riflessi", "Mente", "Tecnica", "Presenza")},
			{Titolo: "Abilità", Campi: valori(0, 10, 2, "Da sommare alla caratteristica", "Armi", "Corpo a corpo", "Furtività", "Percezione", "Persuasione", "Tecnologia", "Guida")},
			{Titolo: "Condizione", Campi: []campoModello{cRis("punti_ferita", "Punti ferita", 35), cNum("protezione", "Protezione", 0, 30, 7, false, "")}},
			sezEquip("Crediti")},
			Tiri: []tiroModello{{Nome: "Riflessi + Armi", Valore: "{riflessi} + {armi}"}, {Nome: "Mente + Percezione", Valore: "{mente} + {percezione}"}},
			PF:   "punti_ferita", Difesa: "protezione", Iniziativa: "1d10 + {riflessi}", Valute: []string{"crediti"}}
	case "passi-esplosivi":
		dado := "Dado: 4, 6, 8, 10 o 12"
		return modelloScheda{Versione: 1, Sezioni: []sezioneModello{sezPersonaggio(),
			{Titolo: "Tratti", Campi: valori(4, 12, 6, dado, "Agilità", "Mente", "Spirito", "Forza", "Resistenza")},
			{Titolo: "Abilità", Campi: valori(4, 12, 4, dado, "Combattimento", "Tiro", "Furtività", "Percezione", "Persuasione", "Conoscenze")},
			{Titolo: "Condizione", Campi: []campoModello{cNum("difesa", "Difesa", 2, 20, 4, false, ""), cNum("robustezza", "Resistenza ai danni", 2, 20, 5, false, ""),
				cRis("ferite", "Ferite", 3), cRis("fatica", "Fatica", 2)}},
			sezEquip("Denaro")}, PF: "ferite", Difesa: "difesa", Iniziativa: "1d20", Valute: []string{"denaro"}}
	case "fate":
		return modelloScheda{Versione: 1, Sezioni: []sezioneModello{
			{Titolo: "Aspetti", Campi: []campoModello{cTesto("concetto", "Concetto"), cTesto("problema", "Problema"), cElenco("aspetti", "Altri aspetti")}},
			{Titolo: "Abilità", Campi: valori(0, 5, 0, "Da 0 a 5", "Atletica", "Combattere", "Furtività", "Percezione", "Empatia", "Volontà", "Conoscenze", "Contatti")},
			{Titolo: "Condizione", Campi: []campoModello{cRis("stress_fisico", "Stress fisico", 3), cRis("stress_mentale", "Stress mentale", 3),
				cElenco("conseguenze", "Conseguenze"), cRis("punti_fato", "Punti fato", 3)}},
			{Titolo: "Talenti e note", Campi: []campoModello{cElenco("talenti", "Talenti"), cArea("note", "Note")}}},
			PF: "stress_fisico", Iniziativa: "{percezione}"}
	case "3d6-sotto":
		return modelloScheda{Versione: 1, Sezioni: []sezioneModello{sezPersonaggio(),
			{Titolo: "Attributi", Campi: valori(3, 20, 10, "", "Forza", "Destrezza", "Intelletto", "Vigore")},
			{Titolo: "Abilità", Campi: valori(3, 20, 10, "Valore da cui tirare sotto", "Combattimento", "Tiro", "Furtività", "Percezione", "Persuasione", "Conoscenze")},
			{Titolo: "Condizione", Campi: []campoModello{cRis("punti_ferita", "Punti ferita", 10), cRis("fatica", "Punti fatica", 10), cNum("difesa", "Difesa", 0, 20, 8, false, "")}},
			sezEquip("Denaro")}, PF: "punti_ferita", Difesa: "difesa", Iniziativa: "{destrezza}", Valute: []string{"denaro"}}
	case "d20-sotto":
		return modelloScheda{Versione: 1, Sezioni: []sezioneModello{sezPersonaggio(),
			{Titolo: "Caratteristiche", Campi: valori(3, 18, 10, "", "Forza", "Costituzione", "Agilità", "Intelligenza", "Volontà", "Fascino")},
			{Titolo: "Abilità", Campi: valori(1, 18, 5, "Valore da cui tirare sotto", "Combattimento", "Tiro", "Furtività", "Percezione", "Persuasione", "Conoscenze")},
			{Titolo: "Condizione", Campi: []campoModello{cRis("punti_ferita", "Punti ferita", 10), cRis("punti_volonta", "Punti volontà", 10), cNum("armatura", "Armatura", 0, 10, 0, false, "")}},
			sezEquip("Denaro")}, PF: "punti_ferita", Difesa: "armatura", Iniziativa: "1d10", Valute: []string{"denaro"}}
	case "narrativi":
		return modelloScheda{Versione: 1, Sezioni: []sezioneModello{sezPersonaggio(),
			{Titolo: "Caratteristiche", Campi: valori(1, 5, 2, "Dadi abilità", "Fisico", "Destrezza", "Intelletto", "Furbizia", "Determinazione", "Carisma")},
			{Titolo: "Abilità", Campi: valori(0, 5, 0, "Gradi: dadi competenza", "Combattimento", "Tiro", "Furtività", "Percezione", "Persuasione", "Conoscenze")},
			{Titolo: "Condizione", Campi: []campoModello{cRis("ferite", "Ferite", 12), cRis("tensione", "Tensione", 10), cNum("difesa", "Difesa", 0, 5, 0, false, "")}},
			sezEquip("Crediti")}, PF: "ferite", Difesa: "difesa", Iniziativa: "{furbizia}", Valute: []string{"crediti"}}
	}
	return modelloBase()
}

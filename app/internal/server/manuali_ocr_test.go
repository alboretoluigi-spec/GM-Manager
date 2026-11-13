package server

import "testing"

// Testo OCR accanto al manuale (2026-11-11): solo il Master, formato «gasterbless-ocr» compattato, elenco, rimozione.
func TestManualiOCR(t *testing.T) {
	tv := nuovoTavolo(t)
	_, out := tv.m.caricaManuale(pdfDiProva(2), "l'avventura")
	id := out["id"].(string)
	ocr := map[string]any{"format": "gasterbless-ocr", "version": 1, "title": "l'avventura", "pages": []any{
		map[string]any{"pdf_page": 2, "blocks": []any{map[string]any{"paragraphs": []any{map[string]any{"lines": []any{
			map[string]any{"text": "BABA GRISA", "bbox": []float64{53.3, 385.4, 145.4, 399.3}},
			map[string]any{"text": "  ", "bbox": []float64{1, 2, 3, 4}},
			map[string]any{"text": "FOR DES COS INT SAG CAR", "bbox": []float64{61, 310, 263, 320}},
		}}}}}},
	}}
	tv.luca.deve(403, "PUT", "/manuali/"+id+"/ocr", ocr)
	tv.m.deve(400, "PUT", "/manuali/"+id+"/ocr", map[string]any{"format": "altro", "pages": []any{}})
	tv.m.deve(404, "PUT", "/manuali/nessuno/ocr", ocr)
	if r := tv.m.deve(200, "PUT", "/manuali/"+id+"/ocr", ocr); r["pagine"] != float64(1) || r["righe"] != float64(2) {
		t.Fatalf("caricamento: %v", r)
	}
	f := tv.m.deve(200, "GET", "/manuali/"+id+"/ocr", nil)
	pag := f["pagine"].([]any)[0].(map[string]any)
	if f["formato"] != "gasterbless-ocr-compatto" || pag["p"] != float64(2) || len(pag["l"].([]any)) != 2 || pag["l"].([]any)[0].([]any)[4] != "BABA GRISA" {
		t.Fatalf("testo compatto: %v", f)
	}
	tv.luca.deve(204, "GET", "/manuali/"+id+"/ocr", nil) // i giocatori: niente (204, non un errore)
	trova := func() any {
		for _, m := range tv.m.deve(200, "GET", "/manuali", nil)["lista"].([]any) {
			if m.(map[string]any)["id"] == id {
				return m.(map[string]any)["ocr"]
			}
		}
		return nil
	}
	if o, _ := trova().(map[string]any); o == nil || o["pagine"] != float64(1) {
		t.Fatalf("elenco dei manuali: %v", trova())
	}
	tv.m.deve(200, "DELETE", "/manuali/"+id+"/ocr", nil)
	tv.m.deve(204, "GET", "/manuali/"+id+"/ocr", nil) // senza testo OCR: niente (204)
	if trova() != nil {
		t.Fatal("dopo la rimozione l'elenco non deve indicare il testo OCR")
	}
}

package server

// Dadi D2 (2026-11-12, scelta dell'utente): le automazioni chiedono conferma prima di tirare, con il numero dei dadi dei
// danni modificabile (per esempio un dado in più per un'arma usata a due mani o un privilegio), poi la pagina mostra i
// dadi 3D con i valori tirati dal server. Per le armi il profilo sta sul server: «dadiDanni» nell'attacco dice quanti dadi
// tirare per ogni componente (0 = come nel profilo). Gli incantesimi e le cure arrivano già dalla pagina con i dadi scelti.
// Il cambio resta scritto nell'esito e nel registro dello scontro, così il Master lo vede.

import (
	"strconv"
	"strings"
)

// conNumeroDadi: i componenti dei danni con il numero di dadi scelto nella conferma. Restituisce i componenti (gli originali
// se non cambia nulla o se si supera il limite di dadi) e la spiegazione del cambio («1d8 → 2d8»).
func conNumeroDadi(l []componenteDanno, n []int) ([]componenteDanno, string) {
	if len(n) == 0 || len(n) > len(l)+5 {
		return l, ""
	}
	out := make([]componenteDanno, len(l))
	copy(out, l)
	note, tot := []string{}, 0
	for i := range out {
		if m := reDadiAttacco.FindStringSubmatch(out[i].Dadi); m != nil && i < len(n) && n[i] > 0 {
			if n[i] > maxDadiComponente {
				return l, ""
			}
			if v, _ := strconv.Atoi(m[1]); v != n[i] {
				nuovo := itoa(n[i]) + "d" + m[2]
				note = append(note, out[i].Dadi+" → "+nuovo)
				out[i].Dadi = nuovo
			}
		}
		tot += dadiDi(out[i].Dadi)
	}
	if len(note) == 0 || tot > maxDadiAttacco {
		return l, ""
	}
	return out, strings.Join(note, ", ")
}

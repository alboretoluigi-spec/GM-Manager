package server

// Griglia della nebbia: celle da 20 px sulla mappa di 1618 x 1048 px (come la mappa originale).
const (
	mappaL        = 1618
	mappaA        = 1048
	cellaNebbia   = 20
	colonneNebbia = (mappaL + cellaNebbia - 1) / cellaNebbia // 81
	righeNebbia   = (mappaA + cellaNebbia - 1) / cellaNebbia // 53
	byteNebbia    = (colonneNebbia*righeNebbia + 7) / 8      // 537
)

// nebbiaVuota: tutta la mappa coperta.
func nebbiaVuota() []byte { return make([]byte, byteNebbia) }

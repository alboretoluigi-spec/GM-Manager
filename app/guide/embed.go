// Package guide incorpora nell'eseguibile le guide in PDF (pulsante «Scarica Guida» della barra in alto).
// Oggi sono provvisorie (copertina con il marchio e «Guida in preparazione»): le guide vere si scrivono più avanti.
package guide

import _ "embed"

// Master è la guida del Master.
//
//go:embed GasterBless_Guida_Master.pdf
var Master []byte

// Giocatori è la guida per i giocatori.
//
//go:embed GasterBless_Guida_Giocatori.pdf
var Giocatori []byte

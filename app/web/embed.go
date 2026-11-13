// Package web contiene le pagine e le risorse dell'app, incorporate nell'eseguibile.
package web

import "embed"

// File contiene tutte le risorse servite dal server (i file .go sono esclusi).
//
//go:embed index.html manifest.webmanifest sw.js ui stile sezioni scheda scheda_libera carta regole dati img font vendor suoni remoto modelli sistemi
var File embed.FS

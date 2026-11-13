//go:build !windows

package main

import "gasterbless/internal/config"

// Fuori da Windows non c'è il servizio: Gaster Bless si avvia come programma normale.
func comeServizio() bool { return false }

func eseguiServizio(config.Config) {}

package rete

import (
	"html"
	"net/http"
	"strings"
)

// paginaChiusa: Gaster Bless da Internet è chiusa. È la pagina che vede chi apre Gaster Bless dal browser o dall'icona
// sulla schermata Home: autonoma (nessuno script), con «Riprova» e, per chi è collegato al Wi-Fi del Master,
// il collegamento alla Gaster Bless di casa.
func paginaChiusa(w http.ResponseWriter, casa string) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	h.Set("Retry-After", "60")
	w.WriteHeader(http.StatusServiceUnavailable)
	link := ""
	if strings.HasPrefix(casa, "https://") {
		link = `<p>Sei a casa del Master, collegato al suo Wi-Fi?</p><p><a class="b" href="` + html.EscapeString(casa) + `/">🏠 Apri Gaster Bless di casa</a></p>`
	}
	w.Write([]byte(`<!doctype html><html lang="it"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="theme-color" content="#15120e"><title>Gaster Bless</title><style>
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:#15120e;color:#eadfca;font:17px/1.5 system-ui,sans-serif;padding:16px;box-sizing:border-box}
main{max-width:440px;text-align:center}h1{font:700 2rem Georgia,serif;letter-spacing:.15em;color:#c9a227;margin:0 0 12px}
.b{display:inline-block;margin:6px 0;padding:12px 20px;border-radius:10px;background:#8a1c1c;color:#fff;text-decoration:none;font-weight:600}
.c{background:transparent;border:1px solid #c9a227;color:#eadfca}p{margin:8px 0}small{color:#b8a98f}</style></head><body><main>
<h1>GASTER BLESS</h1><p><b>La porta di Gaster Bless su Internet ora è chiusa.</b></p>
<p>Il Master la apre poco prima della partita. Riprova tra poco, oppure scrivigli.</p>
<p><a class="b c" href="/">🔄 Riprova</a></p>` + link + `
<p><small>La tua icona e il tuo accesso restano salvati: non devi rifare nulla.</small></p></main></body></html>
`))
}

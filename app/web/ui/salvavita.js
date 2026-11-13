/* Salvavita dell'avvio (2026-11-08, segnalazione di un giocatore: dopo un blocco del server l'app restava per sempre su
   «La nebbia si dirada…»). Script classico, caricato prima dell'app: se dopo 15 secondi la schermata di caricamento
   (elementi con data-caricamento) è ancora lì, o se un file dell'app non si carica, dice cosa succede e offre:
   «Riprova» (ricarica la pagina) e «Riprova da zero» (svuota la copia dell'app salvata sul dispositivo, che dopo un
   blocco del server può essere rimasta a metà tra due versioni, e ricarica). Le schede salvate per l'uso senza
   connessione non si toccano (sono in un altro archivio del browser). */
(function () {
  "use strict";
  var mostrato = false;
  function aiuto(motivo) {
    if (mostrato) return;
    var box = null, el = document.querySelectorAll("[data-caricamento]");
    for (var i = 0; i < el.length; i++) if (el[i].isConnected && el[i].offsetParent !== null) { box = el[i]; break; }
    if (!box) return;
    mostrato = true;
    var p = document.createElement("p");
    p.className = "salvavita-testo";
    p.textContent = motivo || "Gaster Bless non risponde. Il PC del Master potrebbe essere in standby o Gaster Bless si sta riavviando.";
    var r = document.createElement("button");
    r.type = "button"; r.className = "salvavita-tasto"; r.textContent = "↻ Riprova";
    r.onclick = function () { location.reload(); };
    var z = document.createElement("button");
    z.type = "button"; z.className = "salvavita-tasto"; z.textContent = "🧹 Riprova da zero";
    z.title = "Svuota la copia dell'app salvata su questo dispositivo e ricarica";
    z.onclick = function () {
      z.disabled = true; z.textContent = "Pulizia…";
      var lavori = [];
      try { if (navigator.serviceWorker) lavori.push(navigator.serviceWorker.getRegistrations().then(function (rr) { return Promise.all(rr.map(function (x) { return x.unregister(); })); })); } catch (e) { /* ignora */ }
      try { if (window.caches) lavori.push(caches.keys().then(function (k) { return Promise.all(k.map(function (n) { return caches.delete(n); })); })); } catch (e) { /* ignora */ }
      Promise.all(lavori).catch(function () {}).then(function () { location.reload(); });
    };
    var riga = document.createElement("p");
    riga.append(r, " ", z);
    box.append(p, riga);
    // Si riprova anche da soli ogni 20 secondi, finché il server risponde.
    setInterval(function () {
      fetch("/api/v1/io", { cache: "no-store", credentials: "same-origin" }).then(function (x) { if (x.ok) location.reload(); }).catch(function () {});
    }, 20000);
  }
  setTimeout(function () { aiuto(); }, 15000);
  // Un file dell'app (script) che non si carica o ha un errore: l'app non partirà.
  window.addEventListener("error", function (e) {
    var t = e && e.target;
    if (t && t.tagName === "SCRIPT") setTimeout(function () { aiuto("Un file di Gaster Bless non si è caricato."); }, 1500);
    else if (e && e.message && /import|module|export|Unexpected|SyntaxError/i.test(e.message)) setTimeout(function () { aiuto("Gaster Bless non è partita: la copia salvata sul dispositivo potrebbe essere vecchia."); }, 1500);
  }, true);
})();

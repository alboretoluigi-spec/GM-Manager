#!/usr/bin/env bash
# Prove nel browser di Gaster Bless (Playwright, Chromium). Uso, dalla cartella app:
#   bash test/browser/esegui.sh                 (tutte)
#   bash test/browser/esegui.sh regolamenti     (una sola)
# Ogni prova avvia il programma in modalità di prova su una cartella temporanea nuova (porta 7700).
# Le schermate vanno in test/browser/schermate/ (ignorata da Git).
set -u
QUI="$(cd "$(dirname "$0")" && pwd)"
APP="$(cd "$QUI/../.." && pwd)"
TMP="$(mktemp -d)"
mkdir -p "$QUI/schermate" "$TMP/media"
printf 'cartella_media = %s/media\nnomi_certificato = pc-prova.local\n' "$TMP" > "$TMP/prova.conf"
(cd "$APP" && go build -o "$TMP/gasterbless" ./cmd/gasterbless) || exit 1
PROVE="${1:-regolamenti}"
esito=0
for t in $PROVE; do
  pkill -f "$TMP/gasterbless" 2>/dev/null; rm -rf "$TMP/dati"
  "$TMP/gasterbless" -config "$TMP/prova.conf" -prova "$TMP/dati" > "$TMP/server.log" 2>&1 &
  sleep 1.5
  r=$(cd "$QUI" && GB_BIN="$TMP/gasterbless" GB_CONF="$TMP/prova.conf" GB_DATI="$TMP/dati" timeout 300 node "$t.js" 2>&1); codice=$?
  if [ $codice -ne 0 ] || echo "$r" | grep -q "^FALLITO"; then echo "✗ $t"; echo "$r" | tail -15; cp "$TMP/server.log" "$QUI/schermate/server-$t.log"; esito=1; else echo "✓ $t"; echo "$r" | grep -c "^ok" | sed 's/^/  controlli riusciti: /'; fi
done
pkill -f "$TMP/gasterbless" 2>/dev/null
rm -rf "$TMP"
exit $esito

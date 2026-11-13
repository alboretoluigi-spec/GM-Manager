/* ===== INCANTESIMI CONCESSI IN AUTOMATICO =====
 Elenchi di nomi (inglesi, come nell'archivio SPELLS) presi dalle regole 2014 (PHB, supplemento, supplemento).
 Chiave delle sottoclassi = nome italiano usato in CLASSES. Le chiavi numeriche sono il livello
 di CLASSE a cui l'incantesimo si ottiene. modo: 'prep' = sempre preparato, 'con' = conosciuto;
 in entrambi i casi non conta nel limite di incantesimi preparati o conosciuti. */
const SUB_SPELLS={};
/* Circolo della Terra: incantesimi per terreno (3°, 5°, 7°, 9° livello da druido) */
const LAND_SPELLS={};
/* Razze: incantesimi concessi (chiave razza/sottorazza; numero = livello del PERSONAGGIO).
 Si lanciano con i tratti razziali (una volta per riposo lungo per quelli di livello 1+). */
const RACE_SPELLS={};
/* Razze con un trucchetto a scelta (va aggiunto a mano e segnato con ★) */
const RACE_CANTRIP_CHOICE={};

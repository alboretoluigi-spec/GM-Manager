/* Strumenti e professioni (catalogo vuoto: lo riempie il Master). Testi scritti con parole nostre.
 Ogni strumento:
   n      nome; alias: altri nomi da riconoscere nell'equipaggiamento e nelle competenze
   cat    artigiano | kit | gioco | musica
   costo  prezzo indicativo
   uso    come si usa (breve); puo / nonpuo: cosa si può e cosa non si può fare
   abil   abilità collegate: con competenza sia nello strumento sia nell'abilità la prova si tira con vantaggio
   ops    operazioni con tiro: n, car (caratteristica proposta), cd (CD indicativa; null = la decide il Master),
          ab (abilità che dà vantaggio se il personaggio ne ha la competenza)
   fab    lavori di fabbricazione: n, mo (costo dei materiali in monete d'oro), tempo, prodotto
          (generico: true = oggetto a scelta con la regola generale: materiali a metà prezzo, 5 mo di lavoro al giorno)
 Le CD sono indicative: decide sempre il Master. */
const CRAFT_GEN={n:'Crea un oggetto del mestiere',generico:true,tempo:'stabilito dal Master'};
const STRUMENTI=[];
const STRUMENTI_REGOLE='Con la competenza nello strumento aggiungi il bonus di competenza; se hai competenza anche nell\'abilità collegata alla prova tiri con vantaggio. Le operazioni si fanno solo se il personaggio ha la competenza e lo strumento nell\'equipaggiamento. Le CD sono indicative: decide il Master.';
if(typeof window!=='undefined'){window.STRUMENTI=STRUMENTI;window.STRUMENTI_REGOLE=STRUMENTI_REGOLE;}

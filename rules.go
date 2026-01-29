package main

import (
	"html/template"
	"net/http"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

const rulesMarkdown = `## Regler

Spillet starter sådan her: alle spillere stiller sig på række ved
straffekastlinjen. De to forreste spillere har hver en bold.

1. **Spiller 1 skyder først et straffekast.**
   1. Hvis Spiller 1 scorer: fint.
   2. Hvis Spiller 1 brænder: Spiller 1 skal selv hente returen og score
      hurtigst muligt (fra hvor som helst: layup, hopskud osv.).
2. **Spiller 2 må først skyde, når Spiller 1 har skudt sit første straffekast.**
   1. Spiller 2 prøver at score **før** Spiller 1.
   2. Hvis Spiller 2 brænder straffekastet: Spiller 2 henter selv returen og
      prøver at score hurtigst muligt.
3. **Hvem får et bogstav?**
   1. Hvis Spiller 2 scorer først (dvs. overhaler Spiller 1), får Spiller 1 et bogstav.
   2. Bogstaverne gives i rækkefølgen: **H-E-S-T**.
   3. Når en spiller har fået alle fire bogstaver (H, E, S, T), er spilleren **ude** og stiller sig til siden.
   4. Hvis Spiller 1 scorer først, går Spiller 1 bagerst i køen uden at få et bogstav.
4. **Når Spiller 1 scorer**, afleverer Spiller 1 bolden til den næste i køen.
   1. Den nye spiller prøver nu at score **før Spiller 2.**
   2. Hvis den nye spiller scorer først: **Spiller 2 får et bogstav.**
   3. Hvis Spiller 2 scorer først: Spiller 2 går bagerst i køen og afleverer sin
      bold til den næste spiller (så der igen er to bolde i spil).
5. **Spillet fortsætter**, indtil der kun er **to spillere tilbage.**
   1. Når der er to tilbage, spiller de **sudden death**: de skiftes til at
      skyde straffekast.
   2. Den **første der scorer**, mens den anden brænder, **vinder finalen.**
   3. Der er **3 point til vinderen** og **1 point til den anden finalist** i en
      runde.
`

type rulesView struct {
	Path  string
	Title string
	HTML  template.HTML
}

func newRulesView() rulesView {
	// Create markdown parser with extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)

	// Create HTML renderer
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	// Convert markdown to HTML
	htmlContent := markdown.ToHTML([]byte(rulesMarkdown), p, renderer)

	return rulesView{
		Path:  "/rules",
		Title: "Regler",
		HTML:  template.HTML(htmlContent),
	}
}

func (a *App) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	view := newRulesView()
	renderTemplate(w, "layout", view, "templates/layout.html", "templates/rules.html")
}

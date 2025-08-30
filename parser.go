package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/event"
	"github.com/go-echarts/go-echarts/v2/opts"
)

type RawOperation struct {
	OrderDate     string `xml:"order-date"`
	ExecDate      string `xml:"exec-date"`
	Type          string `xml:"type"`
	Description   string `xml:"description"`
	Amount        Amount `xml:"amount"`
	EndingBalance Amount `xml:"ending-balance"`
}

type Amount struct {
	Currency string `xml:"curr,attr"`
	Value    string `xml:",chardata"`
}

type AccountHistory struct {
	XMLName    xml.Name       `xml:"account-history"`
	Operations []RawOperation `xml:"operations>operation"`
}

type Operation struct {
	OrderDate      string
	ExecDate       string
	Type           string
	Amount         Amount
	EndingBalance  Amount
	Kapital        string
	Odsetki        string
	OdsetkiSkarpit string
	OdsetkiKarne   string
	ID             string
}

func parseFloatSafe(s string) float64 {
	val := strings.Replace(strings.TrimSpace(s), ",", ".", 1)
	num, _ := strconv.ParseFloat(val, 64)
	return num
}

func sanitizeFileName(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	return s
}

func monthKey(t time.Time) string {
	return t.Format("2006-01")
}

func main() {
	fileData, err := os.ReadFile("operations.xml")
	if err != nil {
		panic(err)
	}

	var history AccountHistory
	if err := xml.Unmarshal(fileData, &history); err != nil {
		panic(err)
	}

	re := regexp.MustCompile(`KAPITAŁ: ([0-9,]+)\s+ODSETKI: ([0-9,]+)\s+ODSETKI SKAPIT\.: ([0-9,]+)(?:\s+ODSETKI KARNE: ([0-9,]+))?\s+(\d+)`)
	operationsByID := make(map[string][]Operation)

	for _, op := range history.Operations {
		if strings.TrimSpace(op.Type) == "Spłata kredytu" {
			m := re.FindStringSubmatch(op.Description)
			if m != nil {
				odsetkiKarne := "0,00"
				if len(m) >= 5 && m[4] != "" {
					odsetkiKarne = m[4]
				}
				operationsByID[m[5]] = append(operationsByID[m[5]], Operation{
					OrderDate:      op.OrderDate,
					ExecDate:       op.ExecDate,
					Type:           op.Type,
					Amount:         op.Amount,
					EndingBalance:  op.EndingBalance,
					Kapital:        m[1],
					Odsetki:        m[2],
					OdsetkiSkarpit: m[3],
					OdsetkiKarne:   odsetkiKarne,
					ID:             m[5],
				})
			}
		}
	}

	for id, ops := range operationsByID {
		if len(ops) == 0 {
			continue
		}

		sort.Slice(ops, func(i, j int) bool {
			ti, _ := time.Parse("2006-01-02", ops[i].OrderDate)
			tj, _ := time.Parse("2006-01-02", ops[j].OrderDate)
			return ti.Before(tj)
		})

		firstDate, _ := time.Parse("2006-01-02", ops[0].OrderDate)
		lastDate, _ := time.Parse("2006-01-02", ops[len(ops)-1].OrderDate)

		var months []string
		cur := time.Date(firstDate.Year(), firstDate.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(lastDate.Year(), lastDate.Month(), 1, 0, 0, 0, 0, time.UTC)

		for !cur.After(end) {
			months = append(months, monthKey(cur))
			cur = cur.AddDate(0, 1, 0)
		}

		kapitalByMonth := make(map[string]float64)
		odsetkiByMonth := make(map[string]float64)
		var totalKapital, totalOdsetki float64

		for _, op := range ops {
			dt, err := time.Parse("2006-01-02", op.OrderDate)
			if err != nil {
				continue
			}
			mon := monthKey(dt)
			k := parseFloatSafe(op.Kapital)
			o := parseFloatSafe(op.Odsetki)
			kapitalByMonth[mon] += k
			odsetkiByMonth[mon] += o
			totalKapital += k
			totalOdsetki += o
		}

		var kapitalData []opts.BarData
		var odsetkiData []opts.BarData

		var xVals []float64
		var kapitalVals []float64
		var odsetkiVals []float64

		for i, mon := range months {
			kap := kapitalByMonth[mon]
			ods := odsetkiByMonth[mon]
			kapitalData = append(kapitalData, opts.BarData{Value: kap})
			odsetkiData = append(odsetkiData, opts.BarData{Value: ods})

			xVals = append(xVals, float64(i))
			kapitalVals = append(kapitalVals, kap)
			odsetkiVals = append(odsetkiVals, ods)
		}

		barTitle := fmt.Sprintf("Spłaty — ID %s\n", id)
		bar := charts.NewBar()
		actionWithEchartsInstance := `
 document.body.insertAdjacentHTML('beforeend', '<h1 id="kapital_id">Kapital: N/A</h1>');
 document.body.insertAdjacentHTML('beforeend', '<h1 id="odsetki_id">Odsetki: N/A</h1>');
 var formatter = new Intl.NumberFormat("de-DE", {
  style: "currency",
  currency: "PLN"
});
function to2places(x) {

return formatter.format(x)

}

`

		bar.AddJSFuncStrs(opts.FuncOpts(actionWithEchartsInstance))

		bar.SetGlobalOptions(
			charts.WithTitleOpts(opts.Title{Title: barTitle}),
			charts.WithXAxisOpts(opts.XAxis{Name: "Miesiąc"}),
			charts.WithYAxisOpts(opts.YAxis{Name: "Kwota (PLN)"}),
			charts.WithLegendOpts(opts.Legend{Show: opts.Bool(true), Top: "15%"}),
			charts.WithDataZoomOpts(
				opts.DataZoom{
					Type:       "inside",
					XAxisIndex: []int{0},
				},
				opts.DataZoom{
					Type:       "slider",
					XAxisIndex: []int{0},
				},
			),
			charts.WithEventListeners(
				event.Listener{
					EventName: "dataZoom",
					Handler: opts.FuncOpts(`
		  function(params){ 
        const option = this.getOption();
        const series = option.series;
        const dataZooms = option.dataZoom;
        const startValue = dataZooms[0].startValue;
        const endValue = dataZooms[0].endValue;
		
		var sum_kapital = 0;
		var sum_odsetki = 0;
		for (var i = startValue;i<=endValue;i++)
		{
				sum_kapital = series[0].data[i].value + sum_kapital;
				sum_odsetki = series[1].data[i].value + sum_odsetki;

		}
				const kapital_e = document.getElementById('kapital_id');
                        if (kapital_e) {
                            kapital_e.textContent = "Suma kapitalu " + to2places(sum_kapital);
                        }
						const odsetki_e = document.getElementById('odsetki_id');
                        if (odsetki_e) {
                            odsetki_e.textContent = "Suma odsetek " + to2places(sum_odsetki);
                        }

		}`),
				},
			),
		)

		bar.SetXAxis(months).
			AddSeries("Kapitał", kapitalData, charts.WithBarChartOpts(opts.BarChart{Stack: "stack"})).
			AddSeries("Odsetki", odsetkiData, charts.WithBarChartOpts(opts.BarChart{Stack: "stack"}))

		barFile := fmt.Sprintf("splaty_bar_%s.html", sanitizeFileName(id))
		fBar, err := os.Create(barFile)
		if err != nil {
			fmt.Println("Błąd zapisu pliku:", barFile, err)
			continue
		}
		err = bar.Render(fBar)
		fBar.Close()
		if err != nil {
			fmt.Println("Błąd renderowania bar chart dla ID", id, ":", err)
			continue
		}

		fmt.Println("Wygenerowano pliki dla ID:", id, "->", barFile)
	}
}

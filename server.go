package main

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/turfmedia/go-charts/v2"
	"github.com/ziflex/lecho/v3"
)

var (
	gocacheClient = cache.New(5*time.Minute, 10*time.Minute)
	imageType     = "png"
	pfTheme       = charts.ThemeOption{
		IsDarkMode: false,
		BackgroundColor: charts.Color{
			R: 255,
			G: 255,
			B: 255,
			A: 0,
		},
		TextColor: charts.Color{
			R: 0,
			G: 0,
			B: 0,
			A: 109,
		},

		AxisSplitLineColor: charts.Color{
			R: 0,
			G: 0,
			B: 0,
			A: 20,
		},

		AxisStrokeColor: charts.Color{
			R: 0,
			G: 0,
			B: 0,
			A: 50,
		},

		SeriesColors: []charts.Color{
			{
				R: 13,
				G: 136,
				B: 0,
				A: 255,
			},
			{
				R: 255,
				G: 111,
				B: 0,
				A: 255,
			},
		},
	}
)

func writeFile(buf []byte) error {
	tmpPath := "./"
	err := os.MkdirAll(tmpPath, 0700)
	if err != nil {
		return err
	}

	file := filepath.Join(tmpPath, "radar-chart.png")
	err = os.WriteFile(file, buf, 0600)
	if err != nil {
		return err
	}
	return nil
}

func drawText(p *charts.Painter, text string) *charts.Painter {
	return p.Text(text, 103, 109)
}

//  https://charts.services.turfmedia.com

// https://chart.apis.google.com/chart?cht=r&chs=225x225&chd=t:69.12,77,58,61.5,72|-1,-1,-1,-1,72,73,85,50,69.12|-1,-1,-1,-1,-1,-1,68&chco=0D8800,FF6F00,0000FF&chxt=x&chxl=0:|note|mus|reg|ent|pab|jock|dist|sais&chm=t66,33333375,2,6,40,,lt:32:14|B,FF6F0040,1,1,0|B,0D880060,0,1,0|h,BBBBBB44,0,0.1,1|h,CCCCCC44,0,0.2,1|h,CCCCCC44,0,0.3,1|h,CCCCCC44,0,0.4,1|h,99999944,0,0.5,1|h,BBBBBB44,0,0.6,1|h,BBBBBB44,0,0.7,1|h,BBBBBB44,0,0.8,1|h,BBBBBB44,0,0.9,1|h,CCCCCC,0,1,1&chls=1.0

func generateRadarChart(c echo.Context) error {

	// set a key based on URL params
	cacheKey := c.QueryString()

	imageInt, found := gocacheClient.Get(cacheKey)

	if found {
		image := imageInt.([]byte)
		c.Response().Header().Set(echo.HeaderContentType, "image/png")
		c.Response().Header().Set(echo.HeaderContentLength, strconv.Itoa(len(image)))
		c.Response().Header().Set("Cache-Control", "public, max-age=3600")
		if _, err := c.Response().Write(image); err != nil {
			// Handle error properly
			return c.String(http.StatusInternalServerError, "Failed to send chart image")
		}
		return c.NoContent(http.StatusOK)
	}

	// Extract parameters from the query string
	chartType := c.QueryParam("cht")
	sizeString := c.QueryParam("chs")
	dataString := c.QueryParam("chd")
	// colorString := c.QueryParam("chco")
	axisLabelsString := c.QueryParam("chxl")
	// charts.SetDefaultFont()
	// background #FEDABF
	// line #FF9343
	// Validate chart type
	if chartType != "r" {
		return c.String(http.StatusBadRequest, "Unsupported chart type")
	}

	// Parse size
	size := strings.Split(sizeString, "x")
	if len(size) != 2 {
		return c.String(http.StatusBadRequest, "Invalid chart size")
	}
	width, err := strconv.Atoi(size[0])

	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid chart width")
	}

	height, err := strconv.Atoi(size[1])

	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid chart height")
	}

	// Set default width and height
	charts.SetDefaultWidth(width)
	charts.SetDefaultHeight(height)

	rawDataSeries := strings.Split(strings.TrimPrefix(dataString, "t:"), "|")
	var dataSeries [][]float64
	for _, series := range rawDataSeries {
		var seriesData []float64
		for _, dataPoint := range strings.Split(series, ",") {

			value, err := strconv.ParseFloat(dataPoint, 64)
			if err != nil {
				return c.String(http.StatusBadRequest, "Invalid data point")
			}
			seriesData = append(seriesData, value)
		}
		if len(seriesData) > 0 {
			dataSeries = append(dataSeries, seriesData)
		}
	}

	// Keep only first 2 series and for the second series, replace the first number with the same value as the last one
	if len(dataSeries) > 2 {
		dataSeries = dataSeries[:2]
		dataSeries[1][0] = dataSeries[1][len(dataSeries[1])-1]
	}

	// note is the first value of the first series rounded to int
	note := int(dataSeries[0][0])

	// Parse axis labels
	// Assuming axisLabelsString format is "0:|label1|label2|label3"
	_, axisLabelsPart := strings.SplitN(axisLabelsString, ":", 2)[0], strings.SplitN(axisLabelsString, ":", 2)[1]
	axisLabels := strings.Split(axisLabelsPart, "|")

	// trim the first element
	axisLabels = axisLabels[1:]

	imageOption := charts.PNGTypeOption()

	if imageType == "png" {
		imageOption = charts.PNGTypeOption()
	} else if imageType == "svg" {
		imageOption = charts.SVGTypeOption()
	}

	seriesList := make(charts.SeriesList, len(dataSeries))
	for index, series := range dataSeries {

		data := make([]charts.SeriesData, len(series))

		for i, value := range series {
			data[i] = charts.SeriesData{Value: value, Style: charts.Style{
				StrokeWidth: 0.5,
				StrokeColor: charts.Color{
					R: 0,
					G: 0,
					B: 0,
					A: 109,
				},
			}}
		}

		seriesList[index] = charts.Series{
			Data: data,
			Type: charts.ChartTypeRadar,

			Style: charts.Style{
				StrokeWidth: 0.5,
				StrokeColor: charts.Color{
					R: 0,
					G: 0,
					B: 0,
					A: 109,
				},
			},
		}
	}

	p, err := charts.Render(charts.ChartOption{
		SeriesList:      seriesList,
		LineStrokeWidth: 0.5,
	},
		imageOption,
		charts.ThemeOptionFunc("pf"),
		charts.PaddingOptionFunc(charts.Box{Top: 0, Left: 10, Right: 10, Bottom: -20, IsSet: true}),
		charts.RadarIndicatorOptionFunc(axisLabels, []float64{
			100,
			100,
			100,
			100,
			100,
			100,
			100,
			100,
		}),
	)
	if err != nil {
		panic(err)
	}

	p.SetTextStyle(charts.Style{FontSize: 28.0, FontColor: charts.Color{R: 0, G: 0, B: 0, A: 100}})

	strNote := strconv.Itoa(note)

	box := p.MeasureText(strNote)

	w, h := box.Width(), box.Height()

	posX := (225 - w) / 2
	posY := (225-h)/2 + 22

	p.Text(strNote, posX, posY)

	buf, err := p.Bytes()
	if err != nil {
		// Handle error properly, maybe return an HTTP error response
		return c.String(http.StatusInternalServerError, "Failed to generate chart")
	}

	c.Response().Header().Set(echo.HeaderContentType, "image/png")
	c.Response().Header().Set(echo.HeaderContentLength, strconv.Itoa(len(buf)))
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	if _, err := c.Response().Write(buf); err != nil {
		// Handle error properly
		return c.String(http.StatusInternalServerError, "Failed to send chart image")
	}

	gocacheClient.Set(cacheKey, []byte(buf), cache.DefaultExpiration)

	// return c.NoContent(http.StatusOK)
	return nil
}

func main() {
	buildInfo, _ := debug.ReadBuildInfo()

	charts.AddTheme("pf", pfTheme)

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		Level(zerolog.TraceLevel).
		With().
		Timestamp().
		Int("pid", os.Getpid()).
		Str("go_version", buildInfo.GoVersion).
		Logger()

	logger.Info().Msg("Starting server")

	e := echo.New()
	e.Logger = lecho.From(logger)
	e.GET("/chart", generateRadarChart)
	e.Logger.Fatal(e.Start(":8080"))
}

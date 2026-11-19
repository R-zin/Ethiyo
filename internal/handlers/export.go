package handlers

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/gin-gonic/gin"
)

var errInvalidFormat = errors.New("format must be one of: json, geojson, gpx")

// writeDownload sends a file attachment. filename is built from a validated
// bus code (alphanumerics, '-' and '_'), so it is safe in the header.
func writeDownload(c *gin.Context, contentType, filename string, body []byte) {
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, contentType, body)
}

// --- GeoJSON (RFC 7946) ---

type geoJSONGeometry struct {
	Type        string `json:"type"`
	Coordinates any    `json:"coordinates"`
}

type geoJSONFeature struct {
	Type       string          `json:"type"`
	Geometry   geoJSONGeometry `json:"geometry"`
	Properties map[string]any  `json:"properties"`
}

type geoJSONCollection struct {
	Type     string           `json:"type"`
	Features []geoJSONFeature `json:"features"`
}

// renderGeoJSON emits a FeatureCollection: the trail as a LineString (when
// there are at least two points, as RFC 7946 requires) plus one Point per
// fix carrying its speed and timestamp. Positions are [longitude, latitude].
func renderGeoJSON(h *models.BusHistory) []byte {
	fc := geoJSONCollection{Type: "FeatureCollection", Features: []geoJSONFeature{}}

	if len(h.Points) >= 2 {
		line := make([][2]float64, len(h.Points))
		for i, p := range h.Points {
			line[i] = [2]float64{p.Longitude, p.Latitude}
		}
		fc.Features = append(fc.Features, geoJSONFeature{
			Type:     "Feature",
			Geometry: geoJSONGeometry{Type: "LineString", Coordinates: line},
			Properties: map[string]any{
				"bus_code":   h.BusCode,
				"distance_m": h.DistanceMeters,
				"start":      h.Points[0].RecordedAt.UTC().Format(time.RFC3339),
				"end":        h.Points[len(h.Points)-1].RecordedAt.UTC().Format(time.RFC3339),
			},
		})
	}
	for _, p := range h.Points {
		fc.Features = append(fc.Features, geoJSONFeature{
			Type:     "Feature",
			Geometry: geoJSONGeometry{Type: "Point", Coordinates: [2]float64{p.Longitude, p.Latitude}},
			Properties: map[string]any{
				"bus_code":    h.BusCode,
				"speed":       p.Speed,
				"recorded_at": p.RecordedAt.UTC().Format(time.RFC3339),
			},
		})
	}

	out, err := json.Marshal(fc)
	if err != nil { // unreachable: only plain values are marshalled
		slog.Error("geojson export failed", "error", err)
		return []byte(`{"type":"FeatureCollection","features":[]}`)
	}
	return out
}

// --- GPX 1.1 ---

type gpxPoint struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Time string  `xml:"time"`
}

type gpxDoc struct {
	XMLName  xml.Name `xml:"gpx"`
	Version  string   `xml:"version,attr"`
	Creator  string   `xml:"creator,attr"`
	Xmlns    string   `xml:"xmlns,attr"`
	Metadata struct {
		Name string `xml:"name"`
		Time string `xml:"time"`
	} `xml:"metadata"`
	Track struct {
		Name    string `xml:"name"`
		Segment struct {
			Points []gpxPoint `xml:"trkpt"`
		} `xml:"trkseg"`
	} `xml:"trk"`
}

// renderGPX emits a GPX 1.1 track that GPS and mapping tools can import.
func renderGPX(h *models.BusHistory) []byte {
	var doc gpxDoc
	doc.Version = "1.1"
	doc.Creator = "Ethiyo"
	doc.Xmlns = "http://www.topografix.com/GPX/1/1"
	doc.Metadata.Name = "Bus " + h.BusCode + " trail"
	doc.Metadata.Time = time.Now().UTC().Format(time.RFC3339)
	doc.Track.Name = h.BusCode
	doc.Track.Segment.Points = make([]gpxPoint, len(h.Points))
	for i, p := range h.Points {
		doc.Track.Segment.Points[i] = gpxPoint{Lat: p.Latitude, Lon: p.Longitude, Time: p.RecordedAt.UTC().Format(time.RFC3339)}
	}

	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil { // unreachable: only plain values are marshalled
		slog.Error("gpx export failed", "error", err)
		body = nil
	}
	return append([]byte(xml.Header), body...)
}

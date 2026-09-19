package contracts

import "fmt"

// Geometry at the API boundary declares EPSG:4326 GeoJSON longitude/latitude
// order. This is deliberately distinct from CAP polygon latitude/longitude
// ordering; conversion happens at ingestion (P2), never silently here.

// Point is a GeoJSON Point: coordinates are [longitude, latitude].
type Point struct {
	Type        string     `json:"type"` // always "Point"
	Coordinates [2]float64 `json:"coordinates"`
}

// NewPoint validates bounds and returns a Point.
func NewPoint(longitude, latitude float64) (Point, error) {
	if err := checkLonLat(longitude, latitude); err != nil {
		return Point{}, err
	}
	return Point{Type: "Point", Coordinates: [2]float64{longitude, latitude}}, nil
}

func checkLonLat(longitude, latitude float64) error {
	if longitude < -180 || longitude > 180 {
		return fmt.Errorf("longitude %v outside [-180,180]", longitude)
	}
	if latitude < -90 || latitude > 90 {
		return fmt.Errorf("latitude %v outside [-90,90]", latitude)
	}
	return nil
}

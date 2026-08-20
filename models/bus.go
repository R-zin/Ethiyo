package models
type bus struct {
	BusCode string `json:"bus_code"`
	Latitude float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	RouteCode string `json:"route_code"`
}
